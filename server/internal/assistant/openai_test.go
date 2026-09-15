package assistant

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// serving stands a fake OpenAI in front of the provider: the request it
// received is captured, and the canned SSE body is what the provider parses.
func serving(t *testing.T, body string) (*OpenAI, *oaiRequest) {
	t.Helper()
	var got oaiRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &got)
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)

	p := NewOpenAI("test-key", "")
	p.BaseURL = srv.URL
	return p, &got
}

func drainStream(t *testing.T, s Stream) (string, []ToolCall) {
	t.Helper()
	var text string
	var calls []ToolCall
	for {
		d, err := s.Next()
		if err != nil {
			return text, calls
		}
		text += d.Text
		if d.ToolCall != nil {
			calls = append(calls, *d.ToolCall)
		}
	}
}

func TestOpenAIStreamsTextAsItArrives(t *testing.T) {
	p, _ := serving(t, strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"You have "}}]}`,
		`data: {"choices":[{"delta":{"content":"670 kcal left."}}]}`,
		"data: [DONE]",
		"",
	}, "\n\n"))

	stream, err := p.Stream(context.Background(), []Message{{Role: RoleUser, Text: "hi"}}, nil)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	defer stream.Close()

	text, calls := drainStream(t, stream)
	if text != "You have 670 kcal left." {
		t.Errorf("text = %q", text)
	}
	if len(calls) != 0 {
		t.Errorf("unexpected tool calls: %+v", calls)
	}
}

// The arguments never arrive whole. If the fragments were not joined, the loop
// above would be handed unparseable JSON.
func TestOpenAIReassemblesAToolCallFromFragments(t *testing.T) {
	p, _ := serving(t, strings.Join([]string{
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_abc","function":{"name":"get_day","arguments":""}}]}}]}`,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"date\":"}}]}}]}`,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"today\"}"}}]}}]}`,
		"data: [DONE]",
		"",
	}, "\n\n"))

	stream, err := p.Stream(context.Background(), []Message{{Role: RoleUser, Text: "what did I eat"}}, nil)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	defer stream.Close()

	_, calls := drainStream(t, stream)
	if len(calls) != 1 {
		t.Fatalf("got %d calls, want 1: %+v", len(calls), calls)
	}
	if calls[0].ID != "call_abc" || calls[0].Name != "get_day" {
		t.Errorf("call = %+v", calls[0])
	}
	var args struct{ Date string }
	if err := json.Unmarshal(calls[0].Args, &args); err != nil {
		t.Fatalf("arguments did not reassemble into JSON: %q", calls[0].Args)
	}
	if args.Date != "today" {
		t.Errorf("date = %q", args.Date)
	}
}

// Two calls in one turn interleave their fragments, so index is the only thing
// keeping them apart.
func TestOpenAIKeepsConcurrentToolCallsApart(t *testing.T) {
	p, _ := serving(t, strings.Join([]string{
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"a","function":{"name":"get_day","arguments":"{}"}}]}}]}`,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":1,"id":"b","function":{"name":"search_foods","arguments":"{\"query\":"}}]}}]}`,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":1,"function":{"arguments":"\"oat\"}"}}]}}]}`,
		"data: [DONE]",
		"",
	}, "\n\n"))

	stream, err := p.Stream(context.Background(), []Message{{Role: RoleUser, Text: "x"}}, nil)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	defer stream.Close()

	_, calls := drainStream(t, stream)
	if len(calls) != 2 {
		t.Fatalf("got %d calls, want 2: %+v", len(calls), calls)
	}
	if calls[0].Name != "get_day" || calls[1].Name != "search_foods" {
		t.Errorf("calls out of order or wrong: %+v", calls)
	}
	if string(calls[1].Args) != `{"query":"oat"}` {
		t.Errorf("second call args = %q", calls[1].Args)
	}
}

// A tool whose arguments are all optional streams nothing at all, and `null`
// would reach json.Unmarshal in the dispatch above.
func TestOpenAIGivesAnArgumentlessCallAnEmptyObject(t *testing.T) {
	p, _ := serving(t, strings.Join([]string{
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"c1","function":{"name":"get_day"}}]}}]}`,
		"data: [DONE]",
		"",
	}, "\n\n"))

	stream, err := p.Stream(context.Background(), []Message{{Role: RoleUser, Text: "x"}}, nil)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	defer stream.Close()

	_, calls := drainStream(t, stream)
	if len(calls) != 1 || string(calls[0].Args) != "{}" {
		t.Fatalf("args = %q, want {}", calls[0].Args)
	}
}

// A photo has to go up as content parts. A plain turn must not, or every
// message would cost more tokens than it needs to.
func TestOpenAIEncodesAPhotoAsADataURLAndLeavesPlainTurnsAlone(t *testing.T) {
	p, got := serving(t, "data: [DONE]\n\n")

	stream, err := p.Stream(context.Background(), []Message{
		{Role: RoleSystem, Text: "you help someone"},
		{Role: RoleUser, Text: "what is this", Images: []Image{{MediaType: "image/jpeg", Data: "QUJD"}}},
	}, nil)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	stream.Close()

	if len(got.Messages) != 2 {
		t.Fatalf("sent %d messages", len(got.Messages))
	}
	if _, isString := got.Messages[0].Content.(string); !isString {
		t.Errorf("a turn with no photo should send a plain string, got %T", got.Messages[0].Content)
	}
	parts, ok := got.Messages[1].Content.([]any)
	if !ok {
		t.Fatalf("a turn with a photo should send parts, got %T", got.Messages[1].Content)
	}
	if len(parts) != 2 {
		t.Fatalf("got %d parts, want text + image", len(parts))
	}
	image := parts[1].(map[string]any)["image_url"].(map[string]any)["url"].(string)
	if image != "data:image/jpeg;base64,QUJD" {
		t.Errorf("image url = %q", image)
	}
}

// The arguments go up as a JSON string, not as an object. Sending an object is
// rejected, and the tool loop would stall on the second turn of every
// conversation that used a tool.
func TestOpenAIEncodesToolResultsForTheNextTurn(t *testing.T) {
	p, got := serving(t, "data: [DONE]\n\n")

	stream, err := p.Stream(context.Background(), []Message{
		{Role: RoleUser, Text: "what did I eat"},
		{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "call_1", Name: "get_day", Args: json.RawMessage(`{"date":"today"}`)}}},
		{Role: RoleTool, ToolCallID: "call_1", Text: `{"eaten":{"kcal":1630}}`},
	}, nil)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	stream.Close()

	call := got.Messages[1].ToolCalls
	if len(call) != 1 || call[0].Function.Arguments != `{"date":"today"}` {
		t.Fatalf("tool call encoded as %+v", call)
	}
	if call[0].Type != "function" {
		t.Errorf("type = %q", call[0].Type)
	}
	if got.Messages[2].ToolCallID != "call_1" {
		t.Errorf("result is not tied back to the call: %+v", got.Messages[2])
	}
}

func TestOpenAIToolSchemasGoUpVerbatim(t *testing.T) {
	p, got := serving(t, "data: [DONE]\n\n")
	schema := json.RawMessage(`{"type":"object","properties":{"date":{"type":"string"}}}`)

	stream, err := p.Stream(context.Background(), []Message{{Role: RoleUser, Text: "x"}},
		[]Tool{{Name: "get_day", Description: "one day", Schema: schema}})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	stream.Close()

	if len(got.Tools) != 1 {
		t.Fatalf("sent %d tools", len(got.Tools))
	}
	if got.Tools[0].Function.Name != "get_day" || string(got.Tools[0].Function.Parameters) != string(schema) {
		t.Errorf("tool = %+v", got.Tools[0])
	}
}

// A wrong key or a model that does not exist must fail with the reason in it -
// that message is the whole diagnosis, and it only ever reaches the server log.
func TestOpenAIReportsWhyARequestWasRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"error":{"message":"The model 'gpt-nope' does not exist"}}`)
	}))
	defer srv.Close()

	p := NewOpenAI("test-key", "gpt-nope")
	p.BaseURL = srv.URL

	_, err := p.Stream(context.Background(), []Message{{Role: RoleUser, Text: "x"}}, nil)
	if err == nil {
		t.Fatal("a 404 was accepted")
	}
	if !strings.Contains(err.Error(), "gpt-nope") {
		t.Errorf("error does not say what went wrong: %v", err)
	}
}

func TestOpenAIDefaultsToAModelThatCanSeeAndCallTools(t *testing.T) {
	if NewOpenAI("k", "").Model != DefaultOpenAIModel {
		t.Errorf("default model = %q", NewOpenAI("k", "").Model)
	}
	if NewOpenAI("k", "gpt-5.6-terra").Model != "gpt-5.6-terra" {
		t.Error("an explicit model was overridden")
	}
}
