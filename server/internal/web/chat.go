package web

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Meizuno/calories/internal/assistant"
)

// The assistant endpoint.
//
// One POST carries the conversation and streams the reply back as Server-Sent
// Events. SSE rather than a WebSocket because the traffic is one-directional
// once the request is made, and it survives proxies without special handling.
//
// The transcript lives in the browser for now and is sent whole each turn, so
// the limits below are the only thing standing between a long chat and an
// expensive one. When history moves server-side, the server will rebuild it
// instead and these become a floor rather than the whole defence.
const (
	// maxMessages caps the transcript. Old turns matter less than recent ones,
	// so the oldest are dropped rather than the request refused.
	maxMessages = 40
	// maxMessageChars caps a single message, and maxTotalChars the whole
	// conversation. Characters, not tokens: an approximation the server can
	// apply without a tokenizer for whichever model is behind the interface.
	maxMessageChars = 4000
	maxTotalChars   = 24000

	// A photo of a plate is worth a paragraph of description, so the assistant
	// accepts them — but they are the one part of a turn that can be megabytes,
	// and they are charged for by the model as well as carried by us.
	//
	// maxImages caps one turn. Three covers a plate from two angles and the
	// packet it came out of; past that it is a gallery, not a question.
	maxImages = 3
	// maxImageBytes is the DECODED size. The client downscales before sending,
	// so anything near this is a client that did not — refuse it rather than
	// forward it to a model that bills by the pixel.
	maxImageBytes = 2 << 20
	// maxChatBody bounds the whole request, since the counts above are only
	// known after it has been read into memory.
	maxChatBody = 12 << 20
)

// imageTypes is what a provider can be expected to accept. An allowlist rather
// than a prefix check on "image/": a media type is echoed back to the model
// verbatim, and the formats every vision API documents are these four.
var imageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
	"image/gif":  true,
}

// chatImage is a photo as the client sends it.
type chatImage struct {
	MediaType string `json:"mediaType"`
	// Data is base64 with no data: prefix — the client strips it, so the server
	// never has to guess where the payload starts.
	Data string `json:"data"`
}

// chatMessage is one turn as the client sends it. Named rather than inline so
// the validation below, and its tests, can talk about one.
type chatMessage struct {
	Role   string      `json:"role"`
	Text   string      `json:"text"`
	Images []chatImage `json:"images,omitempty"`
}

type chatRequest struct {
	Messages []chatMessage `json:"messages"`
	// Mode is "read" or "write". It only ever takes permissions AWAY: a caller
	// who could log a meal may ask not to be able to, but asking for "write"
	// grants nothing a caller did not already hold.
	Mode string `json:"mode"`
}

// applyMode narrows a caller's scopes to what they asked for this message.
// Read mode drops every writing scope, so the assistant is not offered the
// tool at all — a refusal the model cannot talk itself out of, because the
// tool simply is not there.
func applyMode(scopes []string, mode string) []string {
	if !strings.EqualFold(strings.TrimSpace(mode), "read") {
		return scopes
	}
	out := make([]string, 0, len(scopes))
	for _, s := range scopes {
		if s == "read" {
			out = append(out, s)
		}
	}
	return out
}

// Chat answers one turn of conversation.
func (h *Handlers) Chat(w http.ResponseWriter, r *http.Request) {
	if h.assistant == nil || !h.assistant.Available() {
		writeError(w, http.StatusServiceUnavailable, "assistant_unavailable",
			"the assistant is not configured on this server")
		return
	}

	// Bounded before it is read, not after: a photo makes this the one endpoint
	// where an oversized body is plausible, and the alternative is holding
	// whatever was sent in memory to find out how big it is.
	r.Body = http.MaxBytesReader(w, r.Body, maxChatBody)

	var req chatRequest
	if !decode(w, r, &req) {
		return
	}
	scopes := applyMode(effectiveScopes(r.Context()), req.Mode)
	msgs, err := conversation(req, hasScope(scopes, "add"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_conversation", err.Error())
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		// Without flushing this would buffer the whole reply and arrive as one
		// lump, which defeats the point of streaming.
		apiError(w, r, errors.New("response writer cannot stream"))
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Tell any proxy in front of us not to buffer; Caddy and nginx both honour it.
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ctx := r.Context()
	send := func(e assistant.Event) error {
		// Stop as soon as the caller goes away rather than finishing a reply
		// nobody will read — and paying the model for it.
		if err := ctx.Err(); err != nil {
			return err
		}
		payload, err := json.Marshal(e)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	runErr := h.assistant.Run(ctx, ProfileID(ctx), scopes, msgs, send)
	if runErr == nil || ctx.Err() != nil {
		return
	}

	// The status line is long gone by now, so a failure has to be reported
	// inside the stream. The client translates the code like any other.
	logRequestError(r, "assistant", runErr)
	code := "assistant_failed"
	switch {
	case errors.Is(runErr, assistant.ErrNoProvider):
		code = "assistant_unavailable"
	case errors.Is(runErr, assistant.ErrTurnBudget):
		code = "assistant_gave_up"
	}
	_ = send(assistant.Event{Type: assistant.EventError, Code: code})
}

// conversation validates and trims what the client sent.
//
// Roles other than user and assistant are dropped rather than trusted: the
// system prompt is the server's to write, and a tool result is the server's to
// produce. Accepting either from a client would let it tell the model whatever
// it liked about what the tools returned.
func conversation(req chatRequest, canWrite bool) ([]assistant.Message, error) {
	msgs := make([]assistant.Message, 0, len(req.Messages))
	for _, m := range req.Messages {
		var role assistant.Role
		switch assistant.Role(m.Role) {
		case assistant.RoleUser:
			role = assistant.RoleUser
		case assistant.RoleAssistant:
			role = assistant.RoleAssistant
		default:
			continue
		}
		text := strings.TrimSpace(m.Text)
		if len(text) > maxMessageChars {
			text = text[:maxMessageChars]
		}

		images := make([]assistant.Image, 0, len(m.Images))
		for _, img := range m.Images {
			if role != assistant.RoleUser {
				break // only a person attaches a photo
			}
			if len(images) >= maxImages {
				return nil, fmt.Errorf("at most %d photos per message", maxImages)
			}
			if !imageTypes[strings.ToLower(strings.TrimSpace(img.MediaType))] {
				return nil, errors.New("that image format is not supported")
			}
			// base64 is 4 characters per 3 bytes; check before decoding so an
			// enormous string is refused rather than expanded first.
			if len(img.Data)/4*3 > maxImageBytes {
				return nil, errors.New("that photo is too large")
			}
			if _, err := base64.StdEncoding.DecodeString(img.Data); err != nil {
				return nil, errors.New("that photo could not be read")
			}
			images = append(images, assistant.Image{
				MediaType: strings.ToLower(strings.TrimSpace(img.MediaType)),
				Data:      img.Data,
			})
		}

		// A photo on its own is a question — "what is this?" — so an empty text
		// is only empty when nothing came with it.
		if text == "" && len(images) == 0 {
			continue
		}
		msgs = append(msgs, assistant.Message{Role: role, Text: text, Images: images})
	}
	if len(msgs) == 0 {
		return nil, errors.New("say something first")
	}
	if msgs[len(msgs)-1].Role != assistant.RoleUser {
		return nil, errors.New("the last message must be from you")
	}

	// Drop from the front: the most recent turns carry the most meaning, and
	// the system prompt is prepended afterwards so it is never what gets cut.
	if len(msgs) > maxMessages {
		msgs = msgs[len(msgs)-maxMessages:]
	}
	for total := chars(msgs); total > maxTotalChars && len(msgs) > 1; total = chars(msgs) {
		msgs = msgs[1:]
	}

	// Photos are kept on the newest turn only. The transcript lives in the
	// browser and is re-sent whole every turn, so keeping them would re-upload
	// — and re-bill — every picture ever taken, on every message, forever. The
	// model still has its own earlier description of the photo to refer back
	// to, which is the part that was worth keeping.
	for i := range msgs[:len(msgs)-1] {
		msgs[i].Images = nil
	}

	return append([]assistant.Message{{Role: assistant.RoleSystem, Text: assistant.SystemPrompt(canWrite)}}, msgs...), nil
}

func chars(msgs []assistant.Message) int {
	n := 0
	for _, m := range msgs {
		n += len(m.Text)
	}
	return n
}

func hasScope(scopes []string, want string) bool {
	for _, s := range scopes {
		if s == want {
			return true
		}
	}
	return false
}

// effectiveScopes is what this caller may ask the assistant to do on their
// behalf. A full browser session can do everything the UI can, so it gets the
// lot; a PAT gets exactly what it was issued, and never more — the assistant is
// not a way around a token's limits.
func effectiveScopes(ctx context.Context) []string {
	if IsFull(ctx) {
		return []string{"read", "add"}
	}
	return Scopes(ctx)
}
