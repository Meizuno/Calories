// Package assistant runs a chat turn against a language model that may call
// back into the diary through a small set of tools.
//
// The model vendor sits behind Provider. Everything above it — the tool loop,
// the scope checks, the turn budget, the event stream — is vendor-agnostic, so
// swapping or adding a provider touches one file and no policy.
//
// A Provider is also trivially fakeable, which is the point: the loop, the tool
// dispatch and the refusals are all testable without an API key or a network.
package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
)

// ── conversation ─────────────────────────────────────────────────────────────

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	// RoleTool carries a tool's result back to the model. ToolCallID says which
	// call it answers, because a model may ask for several at once.
	RoleTool Role = "tool"
)

// Image is a picture attached to a turn — a photo of a meal. Data is raw
// base64 with no data: prefix, which is the shape every vision API wants.
//
// Images are never stored: they ride the request they were attached to and are
// gone once it is answered. Nothing to clean up, nothing to leak, and no blob
// store to run — the sibling ai-chat needs one because it keeps history on the
// server, and this does not.
type Image struct {
	MediaType string `json:"mediaType"`
	Data      string `json:"data"`
}

type Message struct {
	Role Role   `json:"role"`
	Text string `json:"text,omitempty"`
	// Images ride a user turn. A provider that cannot see them should say so
	// rather than answering as if the photo were not there.
	Images []Image `json:"images,omitempty"`
	// Set on an assistant turn that asked for tools.
	ToolCalls []ToolCall `json:"toolCalls,omitempty"`
	// Set on a RoleTool message.
	ToolCallID string `json:"toolCallId,omitempty"`
}

// ToolCall is a model's request to run one tool. Providers accumulate whatever
// partial form their wire protocol streams and hand over completed calls, so
// the loop never deals in fragments.
type ToolCall struct {
	ID   string          `json:"id"`
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

// ── tools ────────────────────────────────────────────────────────────────────

// Tool is something the model may invoke on the caller's behalf.
//
// Scope is checked against the caller's own scopes before Run is reached, using
// the same names the HTTP gate uses ("read", "add"). A tool is therefore never
// able to do something its caller could not do directly — the model is not a
// privilege escalation.
type Tool struct {
	Name        string
	Description string
	// Schema is the JSON Schema for Args, handed to the model verbatim.
	Schema json.RawMessage
	Scope  string
	Run    func(ctx context.Context, profileID int64, args json.RawMessage) (any, error)
}

// ── streaming ────────────────────────────────────────────────────────────────

// Delta is one piece of a model's reply: some text, or a completed tool call.
type Delta struct {
	Text     string
	ToolCall *ToolCall
}

// Stream yields deltas until it is exhausted. Next returns io.EOF when the
// model has finished this turn.
type Stream interface {
	Next() (Delta, error)
	Close() error
}

// Provider is one model vendor. The only thing the rest of the package knows
// about OpenAI, Anthropic or anything else.
type Provider interface {
	Name() string
	Stream(ctx context.Context, msgs []Message, tools []Tool) (Stream, error)
}

// ── events out ───────────────────────────────────────────────────────────────

type EventType string

const (
	EventText EventType = "text"
	// EventTool announces a tool that is about to run, so the UI can say what
	// is happening during the pause rather than looking stalled.
	EventTool  EventType = "tool"
	EventError EventType = "error"
	EventDone  EventType = "done"
)

type Event struct {
	Type EventType `json:"type"`
	Text string    `json:"text,omitempty"`
	Tool string    `json:"tool,omitempty"`
	// Code is a stable error code, translated by the client like every other.
	Code string `json:"code,omitempty"`
}

// ── the loop ─────────────────────────────────────────────────────────────────

var (
	// ErrNoProvider means the deployment has no model configured.
	ErrNoProvider = errors.New("no assistant provider configured")
	// ErrTurnBudget means the model kept asking for tools past the cap. It is a
	// guard against a loop that never converges, which would otherwise bill the
	// caller indefinitely.
	ErrTurnBudget = errors.New("assistant used its whole turn budget")
)

// maxTurns bounds how many times the model may go round the tool loop in a
// single request. Each turn is a billed call, so this is a cost ceiling as much
// as a liveness one. Answering a question about a day needs one or two.
const maxTurns = 6

type Service struct {
	provider Provider
	tools    map[string]Tool
	order    []string // stable order, so the model sees a stable tool list
}

func New(p Provider, tools ...Tool) *Service {
	s := &Service{provider: p, tools: make(map[string]Tool, len(tools))}
	for _, t := range tools {
		s.tools[t.Name] = t
		s.order = append(s.order, t.Name)
	}
	return s
}

func (s *Service) Available() bool { return s != nil && s.provider != nil }

// ProviderName is reported to the client so a deployment can be told apart from
// a mock without exposing configuration.
func (s *Service) ProviderName() string {
	if !s.Available() {
		return ""
	}
	return s.provider.Name()
}

// allowed returns the tools this caller may actually use. A caller with only
// "read" simply never learns that log_meal exists, which is a better answer
// than offering it and refusing: a model cannot be tempted by a tool it was
// never shown.
func (s *Service) allowed(scopes []string) []Tool {
	out := make([]Tool, 0, len(s.order))
	for _, name := range s.order {
		t := s.tools[name]
		if t.Scope == "" || hasScope(scopes, t.Scope) {
			out = append(out, t)
		}
	}
	return out
}

func hasScope(scopes []string, want string) bool {
	for _, s := range scopes {
		if s == want {
			return true
		}
	}
	return false
}

// Run drives one request to completion, emitting events as they happen.
//
// `emit` is called from this goroutine only, so a handler can write to an SSE
// response without locking. It returns an error to stop early — a disconnected
// client — and Run gives up promptly rather than finishing a reply nobody will
// read.
func (s *Service) Run(ctx context.Context, profileID int64, scopes []string, msgs []Message, emit func(Event) error) error {
	if !s.Available() {
		return ErrNoProvider
	}
	tools := s.allowed(scopes)
	convo := append([]Message(nil), msgs...)

	for turn := 0; turn < maxTurns; turn++ {
		stream, err := s.provider.Stream(ctx, convo, tools)
		if err != nil {
			return err
		}
		text, calls, err := drain(stream, emit)
		stream.Close()
		if err != nil {
			return err
		}

		// No tools wanted: the model has answered, and we are done.
		if len(calls) == 0 {
			return emit(Event{Type: EventDone})
		}

		convo = append(convo, Message{Role: RoleAssistant, Text: text, ToolCalls: calls})
		for _, call := range calls {
			if err := emit(Event{Type: EventTool, Tool: call.Name}); err != nil {
				return err
			}
			convo = append(convo, Message{
				Role:       RoleTool,
				ToolCallID: call.ID,
				Text:       s.invoke(ctx, profileID, scopes, call),
			})
		}
	}
	return ErrTurnBudget
}

// drain consumes one model turn, streaming its text out as it arrives and
// collecting any tool calls for the caller to run.
func drain(stream Stream, emit func(Event) error) (string, []ToolCall, error) {
	var text string
	var calls []ToolCall
	for {
		d, err := stream.Next()
		if errors.Is(err, errDone) {
			return text, calls, nil
		}
		if err != nil {
			return text, calls, err
		}
		if d.ToolCall != nil {
			calls = append(calls, *d.ToolCall)
		}
		if d.Text != "" {
			text += d.Text
			if err := emit(Event{Type: EventText, Text: d.Text}); err != nil {
				return text, calls, err
			}
		}
	}
}

// invoke runs one tool and renders its result as the JSON the model reads back.
//
// Failures are returned TO THE MODEL rather than aborting the request: "that
// day has no meals" or "you may not do that" is something it can explain, and
// an aborted stream is not. Nothing here leaks an internal error string — the
// model would repeat it to the user.
func (s *Service) invoke(ctx context.Context, profileID int64, scopes []string, call ToolCall) string {
	tool, ok := s.tools[call.Name]
	if !ok {
		return toolError("unknown_tool", "there is no tool by that name")
	}
	// Belt and braces: the model was only shown permitted tools, but a reply
	// naming one anyway must not slip through.
	if tool.Scope != "" && !hasScope(scopes, tool.Scope) {
		return toolError("forbidden", "the person you are helping has not granted this")
	}
	result, err := tool.Run(ctx, profileID, call.Args)
	if err != nil {
		slog.Error("assistant tool failed", "tool", call.Name, "err", err)
		return toolError("failed", "that did not work")
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		slog.Error("assistant tool result not encodable", "tool", call.Name, "err", err)
		return toolError("failed", "that did not work")
	}
	return string(encoded)
}

func toolError(code, message string) string {
	b, _ := json.Marshal(map[string]string{"error": code, "message": message})
	return string(b)
}

// errDone ends a stream. Providers return it rather than io.EOF so a genuine
// transport EOF stays distinguishable from an orderly finish.
var errDone = errors.New("assistant: stream finished")

// Done is what a Provider's Next returns when the turn is over.
func Done() error { return errDone }

// ToolNames is a diagnostic: which tools this caller would be offered.
func (s *Service) ToolNames(scopes []string) []string {
	out := []string{}
	for _, t := range s.allowed(scopes) {
		out = append(out, t.Name)
	}
	return out
}
