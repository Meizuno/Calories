package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// scripted is a Provider whose turns are written out in advance, so the loop,
// the tool dispatch and the refusals can all be driven deterministically with
// no API key and no network.
type scripted struct {
	turns [][]Delta
	// toolsSeen records what the model was offered on each turn, which is how
	// the scope filtering is checked from the outside.
	toolsSeen [][]string
	calls     int
}

func (s *scripted) Name() string { return "scripted" }

func (s *scripted) Stream(_ context.Context, _ []Message, tools []Tool) (Stream, error) {
	names := make([]string, 0, len(tools))
	for _, t := range tools {
		names = append(names, t.Name)
	}
	s.toolsSeen = append(s.toolsSeen, names)

	if s.calls >= len(s.turns) {
		return &mockStream{}, nil // an empty turn ends the loop
	}
	turn := s.turns[s.calls]
	s.calls++
	return &mockStream{deltas: turn}, nil
}

func text(s string) Delta { return Delta{Text: s} }
func call(name, args string) Delta {
	return Delta{ToolCall: &ToolCall{ID: "c1", Name: name, Args: json.RawMessage(args)}}
}

// collect runs a conversation and returns the events, plus the concatenated text.
func collect(t *testing.T, svc *Service, scopes []string, msgs []Message) ([]Event, string, error) {
	t.Helper()
	var events []Event
	var sb strings.Builder
	err := svc.Run(context.Background(), 1, scopes, msgs, func(e Event) error {
		events = append(events, e)
		if e.Type == EventText {
			sb.WriteString(e.Text)
		}
		return nil
	})
	return events, sb.String(), err
}

func ask(s string) []Message { return []Message{{Role: RoleUser, Text: s}} }

// a tool that records what it was given
func recorder(name, scope string, seen *[]string) Tool {
	return Tool{
		Name: name, Scope: scope,
		Schema: json.RawMessage(`{"type":"object"}`),
		Run: func(_ context.Context, profileID int64, args json.RawMessage) (any, error) {
			*seen = append(*seen, string(args))
			return map[string]any{"ok": true, "profileID": profileID}, nil
		},
	}
}

func TestPlainAnswerStreamsAndFinishes(t *testing.T) {
	p := &scripted{turns: [][]Delta{{text("Hello "), text("there")}}}
	svc := New(p)

	events, out, err := collect(t, svc, []string{"read"}, ask("hi"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if out != "Hello there" {
		t.Errorf("text = %q", out)
	}
	if last := events[len(events)-1]; last.Type != EventDone {
		t.Errorf("last event = %q, want done", last.Type)
	}
	if p.calls != 1 {
		t.Errorf("called the model %d times for a question needing no tools", p.calls)
	}
}

func TestToolResultGoesBackToTheModel(t *testing.T) {
	var seen []string
	p := &scripted{turns: [][]Delta{
		{text("Let me look. "), call("get_day", `{"date":"2026-09-15"}`)},
		{text("You had 500 kcal.")},
	}}
	svc := New(p, recorder("get_day", "read", &seen))

	events, out, err := collect(t, svc, []string{"read"}, ask("what did I eat"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(seen) != 1 || !strings.Contains(seen[0], "2026-09-15") {
		t.Fatalf("tool args = %v", seen)
	}
	if !strings.Contains(out, "500 kcal") {
		t.Errorf("final answer missing: %q", out)
	}
	// The UI needs to know a tool ran, so the pause is explained.
	var announced bool
	for _, e := range events {
		if e.Type == EventTool && e.Tool == "get_day" {
			announced = true
		}
	}
	if !announced {
		t.Error("no tool event emitted")
	}
}

func TestScopesDecideWhichToolsExist(t *testing.T) {
	var seen []string
	p := &scripted{turns: [][]Delta{{text("ok")}}}
	svc := New(p, recorder("get_day", "read", &seen), recorder("log_meal", "add", &seen))

	t.Run("a read-only caller is never shown the writing tool", func(t *testing.T) {
		if _, _, err := collect(t, svc, []string{"read"}, ask("hi")); err != nil {
			t.Fatalf("run: %v", err)
		}
		offered := p.toolsSeen[len(p.toolsSeen)-1]
		if len(offered) != 1 || offered[0] != "get_day" {
			t.Errorf("offered %v, want only get_day", offered)
		}
	})

	t.Run("a caller with add sees both", func(t *testing.T) {
		if _, _, err := collect(t, svc, []string{"read", "add"}, ask("hi")); err != nil {
			t.Fatalf("run: %v", err)
		}
		offered := p.toolsSeen[len(p.toolsSeen)-1]
		if len(offered) != 2 {
			t.Errorf("offered %v, want both", offered)
		}
	})
}

// The model is told which tools exist, but being told is not the enforcement.
// A reply naming a tool it was not offered must still be refused.
func TestAToolTheCallerLacksIsRefusedEvenIfNamed(t *testing.T) {
	var seen []string
	p := &scripted{turns: [][]Delta{
		{call("log_meal", `{"meal":"X"}`)},
		{text("done")},
	}}
	svc := New(p, recorder("log_meal", "add", &seen))

	if _, _, err := collect(t, svc, []string{"read"}, ask("log something")); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(seen) != 0 {
		t.Fatalf("the tool RAN for a caller without the scope: %v", seen)
	}
}

func TestUnknownToolIsReportedToTheModelNotFatal(t *testing.T) {
	p := &scripted{turns: [][]Delta{
		{call("no_such_tool", `{}`)},
		{text("sorry about that")},
	}}
	svc := New(p)

	_, out, err := collect(t, svc, nil, ask("hi"))
	if err != nil {
		t.Fatalf("an unknown tool should not abort the request: %v", err)
	}
	if !strings.Contains(out, "sorry") {
		t.Errorf("model never got to recover: %q", out)
	}
}

// A tool that fails must not take the whole request down: the model can explain
// a failure, it cannot explain a dead stream.
func TestAFailingToolIsReportedToTheModel(t *testing.T) {
	p := &scripted{turns: [][]Delta{
		{call("boom", `{}`)},
		{text("that did not work, sorry")},
	}}
	svc := New(p, Tool{
		Name: "boom", Schema: json.RawMessage(`{"type":"object"}`),
		Run: func(context.Context, int64, json.RawMessage) (any, error) {
			return nil, errors.New("pq: connection refused to 10.0.0.5")
		},
	})

	_, out, err := collect(t, svc, nil, ask("hi"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(out, "did not work") {
		t.Errorf("model never recovered: %q", out)
	}
	// The internal error must not have been handed to the model, which would
	// repeat it to the user.
	if strings.Contains(out, "10.0.0.5") {
		t.Error("internal error text leaked into the conversation")
	}
}

func TestTurnBudgetStopsARunawayLoop(t *testing.T) {
	turns := make([][]Delta, 20)
	for i := range turns {
		turns[i] = []Delta{call("get_day", `{}`)}
	}
	var seen []string
	svc := New(&scripted{turns: turns}, recorder("get_day", "read", &seen))

	_, _, err := collect(t, svc, []string{"read"}, ask("loop forever"))
	if !errors.Is(err, ErrTurnBudget) {
		t.Fatalf("err = %v, want ErrTurnBudget", err)
	}
	if len(seen) > maxTurns {
		t.Errorf("ran the tool %d times, want at most %d", len(seen), maxTurns)
	}
}

// A client that goes away must stop the work, not pay for a reply nobody reads.
func TestEmitErrorStopsTheRun(t *testing.T) {
	p := &scripted{turns: [][]Delta{{text("one "), text("two "), text("three")}}}
	svc := New(p)

	stop := errors.New("client went away")
	n := 0
	err := svc.Run(context.Background(), 1, nil, ask("hi"), func(Event) error {
		n++
		if n == 2 {
			return stop
		}
		return nil
	})
	if !errors.Is(err, stop) {
		t.Fatalf("err = %v, want the emit error", err)
	}
	if n != 2 {
		t.Errorf("kept emitting after the client left: %d events", n)
	}
}

func TestWithoutAProviderTheServiceSaysSo(t *testing.T) {
	svc := New(nil)
	if svc.Available() {
		t.Error("Available() with no provider")
	}
	if err := svc.Run(context.Background(), 1, nil, ask("hi"), func(Event) error { return nil }); !errors.Is(err, ErrNoProvider) {
		t.Errorf("err = %v, want ErrNoProvider", err)
	}
}

// The mock cannot see. What it can do is prove a photo reached the provider at
// all, which is the only part of the upload path that is testable before a
// vision provider exists.
func TestMockReportsWhatArrivedAndDoesNotPretendToSee(t *testing.T) {
	m := &MockProvider{}
	stream, err := m.Stream(context.Background(), []Message{{
		Role:   RoleUser,
		Text:   "what is this",
		Images: []Image{{MediaType: "image/jpeg", Data: strings.Repeat("A", 4096)}},
	}}, nil)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	defer stream.Close()

	var b strings.Builder
	for {
		d, err := stream.Next()
		if err != nil {
			break
		}
		b.WriteString(d.Text)
	}
	got := b.String()
	for _, want := range []string{"1 photo", "cannot see", "image/jpeg", "3 KB"} {
		if !strings.Contains(got, want) {
			t.Errorf("reply does not mention %q:\n%s", want, got)
		}
	}
}
