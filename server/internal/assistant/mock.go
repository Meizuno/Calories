package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// MockProvider answers without calling anyone, so the chat UI can be built and
// demonstrated with no API key and no spend. Selected with ASSISTANT=mock.
//
// It is not trying to be clever. It streams a canned reply a word at a time so
// the streaming path is exercised properly, and it will run a tool on request —
// send "/get_day", "/get_stats 2026-09-01 2026-09-15" or "/search_foods oat" —
// so the tool round trip can be seen end to end without a model deciding to
// make one.
type MockProvider struct {
	// Delay between words. Zero in tests, a few tens of ms in dev so the stream
	// looks like a stream rather than one lump.
	Delay time.Duration
}

func (m *MockProvider) Name() string { return "mock" }

func (m *MockProvider) Stream(_ context.Context, msgs []Message, tools []Tool) (Stream, error) {
	last := ""
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == RoleUser {
			last = strings.TrimSpace(msgs[i].Text)
			break
		}
	}

	// A photo arrived: say what reached the server, so the upload path can be
	// checked end to end without a model that can actually see.
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == RoleUser && len(msgs[i].Images) > 0 {
			return words(describeImages(msgs[i].Images), m.Delay), nil
		}
		if msgs[i].Role == RoleUser {
			break
		}
	}

	// A tool result is already in hand: summarise instead of calling again, or
	// the loop would never end.
	if len(msgs) > 0 && msgs[len(msgs)-1].Role == RoleTool {
		return words("Here is what I found: "+compact(msgs[len(msgs)-1].Text), m.Delay), nil
	}

	if strings.HasPrefix(last, "/") {
		if call := parseMockCommand(last, tools); call != nil {
			return &mockStream{deltas: []Delta{{ToolCall: call}}}, nil
		}
		return words("I do not know that command. Try /get_day, /get_stats, /search_foods or /log_meal.", m.Delay), nil
	}

	return words(showcase, m.Delay), nil
}

// showcase is the mock's standing reply. It deliberately uses every element the
// Markdown renderer supports, so the prose components can be checked — spacing,
// dark mode, table alignment, a blocked link — without an API key or a bill.
// If a component is added, add it here too, or nothing will ever exercise it.
var showcase = strings.Join([]string{
	"## Mock assistant",
	"",
	"No model was called. This reply exists to render **every** prose component,",
	"so the styling can be checked without a provider.",
	"",
	"### Today",
	"",
	"| Meal | kcal | Protein |",
	"| --- | ---: | ---: |",
	"| Snidane | 426 | 15 g |",
	"| Svacina | 192 | 23 g |",
	"",
	"Short on protein: **38 g of 180**. Some options:",
	"",
	"- Two eggs, *about* 172 kcal",
	"- Skyr 150 g, 90 kcal",
	"- ~~Chocolate~~ probably not that one",
	"",
	"What happens next:",
	"",
	"1. Pick something",
	"2. Tell me what you ate",
	"3. Switch to `write` mode and I will log it",
	"",
	"> Nothing after 20:00 is recorded yet.",
	"",
	"---",
	"",
	"The request behind this page:",
	"",
	"```",
	"POST /api/v1/chat",
	`{"mode":"write","messages":[...]}`,
	"```",
	"",
	"Links are checked before they are drawn: [this one is safe](https://example.com)",
	"and [this one is not](javascript:alert(1)) — the second renders as plain text.",
	"",
	"Set `ASSISTANT_API_KEY` and switch `ASSISTANT` to a real provider. Meanwhile the",
	"tools do work: try /get_day, /get_stats or /search_foods.",
}, "\n")

// parseMockCommand turns "/get_stats 2026-09-01 2026-09-15" into a tool call,
// but only for a tool the caller was actually offered — so the mock cannot be
// used to reach past a scope check.
func parseMockCommand(line string, tools []Tool) *ToolCall {
	fields := strings.Fields(strings.TrimPrefix(line, "/"))
	if len(fields) == 0 {
		return nil
	}
	name := fields[0]
	var found bool
	for _, t := range tools {
		if t.Name == name {
			found = true
			break
		}
	}
	if !found {
		return nil
	}

	args := map[string]any{}
	switch name {
	case "get_day":
		if len(fields) > 1 {
			args["date"] = fields[1]
		}
	case "get_stats":
		if len(fields) > 2 {
			args["from"], args["to"] = fields[1], fields[2]
		} else {
			args["from"], args["to"] = "yesterday", "today"
		}
	case "search_foods":
		if len(fields) > 1 {
			args["query"] = strings.Join(fields[1:], " ")
		}
	case "log_meal":
		// Deliberately a fixed, obviously-fake meal: the mock must never look
		// like it understood a real instruction to write something.
		args["meal"] = "Mock meal"
		args["entries"] = []map[string]any{
			{"name": "Mock item", "quantity": 100, "unit": "g", "kcal": 100, "carb": 10, "protein": 5, "fat": 3},
		}
	}
	raw, _ := json.Marshal(args)
	return &ToolCall{ID: "mock-1", Name: name, Args: raw}
}

// describeImages reports what actually arrived. The mock cannot see a photo —
// no model is called at all — so it says so plainly rather than inventing a
// meal, which would make a broken vision provider look like a working one.
func describeImages(images []Image) string {
	var b strings.Builder
	b.WriteString("I received ")
	b.WriteString(strconv.Itoa(len(images)))
	if len(images) == 1 {
		b.WriteString(" photo")
	} else {
		b.WriteString(" photos")
	}
	b.WriteString(", but this is the **mock assistant** and it cannot see them.\n\n")
	for i, img := range images {
		// base64 is 4 characters per 3 bytes.
		kb := len(img.Data) * 3 / 4 / 1024
		fmt.Fprintf(&b, "%d. `%s`, about %d KB\n", i+1, img.MediaType, kb)
	}
	b.WriteString("\nSwitch `ASSISTANT` to a provider that can read images and it will tell you what is on the plate.")
	return b.String()
}

// compact shortens a tool result so the canned summary stays readable.
func compact(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 400 {
		return s[:400] + "…"
	}
	return s
}

func words(text string, delay time.Duration) Stream {
	parts := strings.SplitAfter(text, " ")
	deltas := make([]Delta, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			deltas = append(deltas, Delta{Text: p})
		}
	}
	return &mockStream{deltas: deltas, delay: delay}
}

type mockStream struct {
	deltas []Delta
	i      int
	delay  time.Duration
}

func (s *mockStream) Next() (Delta, error) {
	if s.i >= len(s.deltas) {
		return Delta{}, Done()
	}
	if s.delay > 0 {
		time.Sleep(s.delay)
	}
	d := s.deltas[s.i]
	s.i++
	return d, nil
}

func (s *mockStream) Close() error { return nil }
