package assistant

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAI talks to the Chat Completions endpoint.
//
// Written against the HTTP API rather than a vendor SDK on purpose. This is one
// request shape and one streaming format, both stable for years, and the
// official Go SDK has repeatedly lagged the models worth using. A dependency
// here would buy nothing and would be the thing blocking an upgrade.
//
// Everything vendor-specific lives in this file. The loop, the scopes and the
// tool dispatch above it do not know which model answered.
type OpenAI struct {
	Key   string
	Model string
	// BaseURL allows a compatible gateway - or a test server - to stand in.
	BaseURL string
	HTTP    *http.Client
	// ReasoningEffort is sent as reasoning_effort when set.
	//
	// It has to be "none" on this endpoint: the 5.6 family reasons by default,
	// and chat completions refuses function tools together with reasoning -
	// "use /v1/responses or set reasoning_effort to none". Since the entire
	// design is a tool loop, tools win and reasoning goes.
	//
	// Clear it (ASSISTANT_REASONING=) for a model that predates the parameter,
	// which would reject it as unknown rather than ignore it.
	ReasoningEffort string
}

// DefaultOpenAIModel is the cheapest current model that does BOTH vision and
// tool calling, which is the minimum this assistant needs. Deliberately not the
// flagship: a food diary asks for a plate identified and some arithmetic
// checked, not for frontier reasoning.
const DefaultOpenAIModel = "gpt-5.6-luna"

func NewOpenAI(key, model string) *OpenAI {
	if model == "" {
		model = DefaultOpenAIModel
	}
	return &OpenAI{
		Key:             key,
		Model:           model,
		BaseURL:         "https://api.openai.com/v1",
		ReasoningEffort: "none",
		// Generous, and a backstop only: the request context is what actually
		// ends a turn when the person closes the tab. Without any timeout a
		// half-open connection would hold the handler open indefinitely.
		HTTP: &http.Client{Timeout: 3 * time.Minute},
	}
}

func (o *OpenAI) Name() string { return "openai" }

// ── the wire ─────────────────────────────────────────────────────────────────

type oaiRequest struct {
	Model           string       `json:"model"`
	Stream          bool         `json:"stream"`
	Messages        []oaiMessage `json:"messages"`
	Tools           []oaiTool    `json:"tools,omitempty"`
	ReasoningEffort string       `json:"reasoning_effort,omitempty"`
}

// Content is `any` because it is either a plain string or a list of parts, and
// which one matters: a string for ordinary turns, parts only when there is a
// picture. Sending parts everywhere works but costs tokens and makes a logged
// transcript much harder to read.
type oaiMessage struct {
	Role       string        `json:"role"`
	Content    any           `json:"content"`
	ToolCalls  []oaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
}

type oaiToolCall struct {
	ID       string      `json:"id"`
	Type     string      `json:"type"`
	Function oaiFunction `json:"function"`
}

type oaiFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type oaiTool struct {
	Type     string     `json:"type"`
	Function oaiToolDef `json:"function"`
}

type oaiToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// encode turns our vendor-neutral conversation into theirs.
func encode(msgs []Message) []oaiMessage {
	out := make([]oaiMessage, 0, len(msgs))
	for _, m := range msgs {
		om := oaiMessage{Role: string(m.Role), Content: m.Text, ToolCallID: m.ToolCallID}

		if len(m.Images) > 0 {
			parts := make([]any, 0, len(m.Images)+1)
			if m.Text != "" {
				parts = append(parts, map[string]any{"type": "text", "text": m.Text})
			}
			for _, img := range m.Images {
				// A data URL, not a link back to us: the model must not need to
				// reach this server, which may be behind a VPN, on a laptop, or
				// simply not addressable from the internet.
				parts = append(parts, map[string]any{
					"type":      "image_url",
					"image_url": map[string]any{"url": "data:" + img.MediaType + ";base64," + img.Data},
				})
			}
			om.Content = parts
		}

		for _, c := range m.ToolCalls {
			args := string(c.Args)
			if strings.TrimSpace(args) == "" {
				args = "{}"
			}
			om.ToolCalls = append(om.ToolCalls, oaiToolCall{
				ID:   c.ID,
				Type: "function",
				// Arguments go up as a JSON *string*, not as an object.
				Function: oaiFunction{Name: c.Name, Arguments: args},
			})
		}
		out = append(out, om)
	}
	return out
}

func encodeTools(tools []Tool) []oaiTool {
	out := make([]oaiTool, 0, len(tools))
	for _, t := range tools {
		out = append(out, oaiTool{
			Type:     "function",
			Function: oaiToolDef{Name: t.Name, Description: t.Description, Parameters: t.Schema},
		})
	}
	return out
}

// ── the call ─────────────────────────────────────────────────────────────────

func (o *OpenAI) Stream(ctx context.Context, msgs []Message, tools []Tool) (Stream, error) {
	body, err := json.Marshal(oaiRequest{
		Model:           o.Model,
		Stream:          true,
		Messages:        encode(msgs),
		Tools:           encodeTools(tools),
		ReasoningEffort: o.ReasoningEffort,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.Key)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := o.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		// Their message names the model that does not exist or the key that is
		// wrong, which is the whole diagnosis. It goes to the server log; the
		// caller only ever sees the generic code, so none of this reaches a
		// browser.
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<10))
		resp.Body.Close()
		return nil, fmt.Errorf("openai: %s: %s", resp.Status, strings.TrimSpace(string(detail)))
	}

	return &oaiStream{body: resp.Body, r: bufio.NewReader(resp.Body), calls: map[int]*ToolCall{}}, nil
}

// oaiStream turns their chunks into our deltas.
//
// Text is handed over the moment it arrives. Tool calls cannot be: the name and
// the arguments stream in fragments across many chunks, so they are accumulated
// by index and released only once the turn is complete. That is what lets the
// loop above deal in whole calls and never in fragments.
type oaiStream struct {
	body io.ReadCloser
	r    *bufio.Reader

	calls map[int]*ToolCall
	order []int // first-seen order, so calls run in the order they were asked for

	ready    []ToolCall
	finished bool
}

func (s *oaiStream) Close() error { return s.body.Close() }

func (s *oaiStream) Next() (Delta, error) {
	for {
		// Completed tool calls, one at a time, before the turn is declared over.
		if len(s.ready) > 0 {
			call := s.ready[0]
			s.ready = s.ready[1:]
			return Delta{ToolCall: &call}, nil
		}
		if s.finished {
			return Delta{}, Done()
		}

		// ReadString, not a Scanner: a chunk carrying a long tool argument can
		// exceed a Scanner's default token size, and that would silently
		// truncate a call rather than fail loudly.
		line, err := s.r.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				s.finish()
				continue
			}
			return Delta{}, err
		}

		line = strings.TrimRight(line, "\r\n")
		if !strings.HasPrefix(line, "data:") {
			continue // comments, keep-alives, and the blank line between events
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			s.finish()
			continue
		}

		var chunk oaiChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue // one unreadable frame is not worth losing a reply over
		}

		var text string
		for _, ch := range chunk.Choices {
			for _, tc := range ch.Delta.ToolCalls {
				s.accumulate(tc.Index, tc.ID, tc.Function.Name, tc.Function.Arguments)
			}
			text += ch.Delta.Content
		}
		if text != "" {
			return Delta{Text: text}, nil
		}
	}
}

type oaiChunk struct {
	Choices []struct {
		Delta struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    int         `json:"index"`
				ID       string      `json:"id"`
				Function oaiFunction `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`
}

// accumulate folds one fragment into the call it belongs to. Every field is
// appended rather than assigned: the id and the name usually arrive whole in
// the first fragment, but the arguments never do.
func (s *oaiStream) accumulate(index int, id, name, args string) {
	call, ok := s.calls[index]
	if !ok {
		call = &ToolCall{}
		s.calls[index] = call
		s.order = append(s.order, index)
	}
	call.ID += id
	call.Name += name
	call.Args = append(call.Args, args...)
}

// finish releases whatever tool calls were accumulated and marks the turn over.
func (s *oaiStream) finish() {
	s.finished = true
	for _, i := range s.order {
		call := s.calls[i]
		if call.Name == "" {
			continue // a fragment that never became a call
		}
		// A tool with no required arguments streams nothing at all, and the
		// dispatch above would then hand `null` to json.Unmarshal.
		if len(bytes.TrimSpace(call.Args)) == 0 {
			call.Args = json.RawMessage("{}")
		}
		if call.ID == "" {
			// The id is echoed back with the result to say which call it
			// answers, so invent one rather than send an empty string.
			call.ID = fmt.Sprintf("call_%d", i)
		}
		s.ready = append(s.ready, *call)
	}
	s.order = nil
}
