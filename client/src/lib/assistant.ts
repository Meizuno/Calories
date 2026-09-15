import { ApiError } from "./http";
import { toWire } from "./image";

// Talking to the assistant.
//
// EventSource cannot POST, and the conversation has to go up with the request,
// so the stream is read off fetch() by hand. That also gives us an
// AbortController, which is how closing the panel or pressing stop actually
// stops the work — and the bill — rather than just hiding the answer.

/** What the assistant may do this turn. "read" removes its ability to log. */
export type ChatMode = "read" | "write";

export interface ChatMessage {
  role: "user" | "assistant";
  text: string;
  /** Photos attached to this turn, as data URLs. Present on user turns only. */
  images?: string[];
}

/** What the server streams back. Mirrors assistant.Event on the Go side. */
export type ChatEvent =
  | { type: "text"; text: string }
  | { type: "tool"; tool: string }
  | { type: "error"; code?: string }
  | { type: "done" };

export interface ChatHandlers {
  onText(chunk: string): void;
  onTool(tool: string): void;
}

/**
 * Send a conversation and stream the reply.
 *
 * Resolves when the model has finished. Rejects with an ApiError carrying the
 * server's code — either an HTTP failure before the stream opened, or an error
 * event inside it, since by then the status line is long gone.
 */
export async function chat(
  messages: ChatMessage[],
  mode: ChatMode,
  handlers: ChatHandlers,
  signal: AbortSignal,
): Promise<void> {
  const res = await fetch("/api/v1/chat", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    // Photos go up as {mediaType, data} rather than as the data URL they are
    // held in, so the server is handed the two fields a provider wants instead
    // of a string it would have to pull apart. The server keeps them on the
    // newest turn only, but sending the older ones would still mean uploading
    // them — so they are dropped here, where the upload is paid for.
    body: JSON.stringify({
      mode,
      messages: messages.map((m, i) => ({
        role: m.role,
        text: m.text,
        images: i === messages.length - 1 ? (m.images ?? []).map(toWire).filter(Boolean) : undefined,
      })),
    }),
    signal,
  });

  if (!res.ok || !res.body) {
    const body = (await res.text()).trim();
    let code = "unknown";
    let message = body || `Request failed (${res.status})`;
    try {
      const parsed = JSON.parse(body) as { code?: string; message?: string };
      if (parsed.code) code = parsed.code;
      if (parsed.message) message = parsed.message;
    } catch {
      /* a plain-text body is used as-is */
    }
    throw new ApiError(res.status, message, code);
  }

  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  // Chunks arrive on no particular boundary, so hold the tail until a blank
  // line proves an event is complete.
  let buffer = "";
  let failed: string | null = null;

  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });

    let split: number;
    while ((split = buffer.indexOf("\n\n")) !== -1) {
      const frame = buffer.slice(0, split);
      buffer = buffer.slice(split + 2);
      const line = frame.split("\n").find((l) => l.startsWith("data: "));
      if (!line) continue;

      let event: ChatEvent;
      try {
        event = JSON.parse(line.slice(6)) as ChatEvent;
      } catch {
        continue; // a frame we cannot read is not worth killing the reply over
      }
      if (event.type === "text") handlers.onText(event.text);
      else if (event.type === "tool") handlers.onTool(event.tool);
      else if (event.type === "error") failed = event.code ?? "unknown";
      else if (event.type === "done") return;
    }
  }

  if (failed) throw new ApiError(500, "assistant failed", failed);
}
