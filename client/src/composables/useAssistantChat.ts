import { computed, ref } from "vue";
import { chat, type ChatMessage, type ChatMode } from "../lib/assistant";
import { ApiError } from "../lib/http";
import { t } from "../lib/i18n";
import { ImageError, prepareImage, type Attachment } from "../lib/image";

const STORE_KEY = "calories.assistant";
const MODE_KEY = "calories.assistant.mode";
/** Keep the tail only: the server trims anyway, and there is no reason to fill
 *  someone's storage with a conversation nobody will scroll back to. */
const KEEP = 40;
/** Photos waiting to go with the next message. The server takes three. */
const MAX_ATTACHMENTS = 3;

function restoreMode(): ChatMode {
  try {
    // Read is the safe default: an assistant that can write to your diary
    // should be something you turned on, not something you discover.
    return localStorage.getItem(MODE_KEY) === "write" ? "write" : "read";
  } catch {
    return "read";
  }
}

function restore(): ChatMessage[] {
  try {
    const raw = localStorage.getItem(STORE_KEY);
    const parsed = raw ? (JSON.parse(raw) as ChatMessage[]) : [];
    return Array.isArray(parsed) ? parsed.filter((m) => m && typeof m.text === "string") : [];
  } catch {
    return []; // a private window, cleared storage, or something we did not write
  }
}

/**
 * The conversation, its state machine and its persistence.
 *
 * Separated from the view so the page stays presentational — and so a second
 * surface (a panel, a widget) could ever render the same conversation without a
 * second copy of the logic to keep in step.
 *
 * The transcript lives in this browser for now, which is why the whole thing
 * goes up with every turn and the server trims it to a budget.
 */
export function useAssistantChat() {
  const messages = ref<ChatMessage[]>(restore());
  /** Picked but not yet sent. */
  const attachments = ref<Attachment[]>([]);
  const mode = ref<ChatMode>(restoreMode());
  const draft = ref("");
  const busy = ref(false);
  /** What the assistant is doing right now, shown while it is between words. */
  const activity = ref("");
  const errorCode = ref("");
  /** The reply as it arrives, before it settles into `messages`. */
  const streaming = ref("");

  let abort: AbortController | null = null;
  // A photo on its own is a question — "what is this?" — so it is enough to
  // send without any words.
  const canSend = computed(
    () => (draft.value.trim().length > 0 || attachments.value.length > 0) && !busy.value,
  );

  function persist() {
    try {
      // Photos are stripped before saving. localStorage is about five megabytes
      // for the whole origin and one downscaled photo is a few hundred
      // kilobytes of base64, so keeping them would fill it within a handful of
      // meals — and a quota error here loses the ENTIRE transcript, not just
      // the picture. They stay in memory for as long as the tab is open, which
      // is where anyone is going to look at them.
      const light = messages.value.slice(-KEEP).map(({ role, text }) => ({ role, text }));
      localStorage.setItem(STORE_KEY, JSON.stringify(light));
    } catch {
      /* storage is a convenience here, never a requirement */
    }
  }

  /**
   * Take files from a picker, a camera or a paste.
   *
   * Each is downscaled and re-encoded before it is held, so what sits in memory
   * is what will be sent — the preview cannot look like a different photo from
   * the one that goes up, and the multi-megabyte original is released.
   */
  async function attach(files: Iterable<File>) {
    for (const file of files) {
      if (attachments.value.length >= MAX_ATTACHMENTS) {
        errorCode.value = "image_too_many";
        return;
      }
      try {
        attachments.value = [...attachments.value, await prepareImage(file)];
      } catch (e) {
        errorCode.value = e instanceof ImageError ? e.code : "unknown";
        return; // one bad file stops the batch: the rest are probably the same
      }
    }
  }

  function detach(id: string) {
    attachments.value = attachments.value.filter((a) => a.id !== id);
  }

  /** Fold whatever has streamed so far into the transcript. */
  function settle() {
    if (streaming.value.trim()) {
      messages.value = [...messages.value, { role: "assistant", text: streaming.value.trim() }];
    }
  }

  async function send(onChange?: () => void) {
    const text = draft.value.trim();
    if ((!text && !attachments.value.length) || busy.value) return;

    const images = attachments.value.map((a) => a.dataUrl);
    messages.value = [...messages.value, { role: "user", text, ...(images.length ? { images } : {}) }];
    attachments.value = [];
    draft.value = "";
    errorCode.value = "";
    streaming.value = "";
    activity.value = "";
    busy.value = true;
    persist();
    onChange?.();

    abort = new AbortController();
    try {
      await chat(
        messages.value,
        mode.value,
        {
          onText(chunk) {
            streaming.value += chunk;
            activity.value = "";
            onChange?.();
          },
          onTool(tool) {
            // Name the tool rather than show a generic spinner: "reading your
            // diary" explains the pause, and makes it visible when the
            // assistant reads versus when it writes.
            activity.value = t(`assistant.tool.${tool}`);
            onChange?.();
          },
        },
        abort.signal,
      );
      settle();
    } catch (e) {
      // An abort is the person pressing stop; keep whatever had arrived.
      if (e instanceof DOMException && e.name === "AbortError") settle();
      else errorCode.value = e instanceof ApiError ? e.code || "unknown" : "unknown";
    } finally {
      streaming.value = "";
      activity.value = "";
      busy.value = false;
      abort = null;
      persist();
      onChange?.();
    }
  }

  /** Stops the request, which stops the work and the bill — not just the view. */
  function stop() {
    abort?.abort();
  }

  function clear() {
    stop();
    messages.value = [];
    attachments.value = [];
    errorCode.value = "";
    persist();
  }

  function ask(text: string, onChange?: () => void) {
    draft.value = text;
    return send(onChange);
  }

  function setMode(next: ChatMode) {
    mode.value = next;
    try {
      localStorage.setItem(MODE_KEY, next);
    } catch {
      /* the choice just does not survive a reload */
    }
  }

  return {
    messages, draft, busy, activity, errorCode, streaming, canSend, mode, setMode,
    attachments, attach, detach,
    send, stop, clear, ask,
  };
}
