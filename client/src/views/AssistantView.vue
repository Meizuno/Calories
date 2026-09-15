<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from "vue";
import MarkdownText from "../components/MarkdownText.vue";
import { parseMarkdown } from "../lib/markdown";
import { useAssistantChat } from "../composables/useAssistantChat";
import { useUiSize } from "../composables/useUiSize";
import { ACCEPTED_IMAGES } from "../lib/image";
import { t } from "../lib/i18n";

// The assistant as a page: a conversation gets long, and a long conversation
// wants the width.
//
// The shape follows the sibling ai-chat project: one narrow column, the user's
// words in a quiet bubble on the right, and the assistant's as plain prose with
// no bubble at all. Boxing both sides makes a conversation read as a ledger;
// leaving the answer unboxed lets it read as writing, which is what it is.
const {
  messages, draft, busy, activity, errorCode, streaming, canSend, mode, setMode,
  attachments, attach, detach,
  send, stop, clear, ask,
} = useAssistantChat();
const { inline, control } = useUiSize();

// The file input is hidden and driven from the button beside it: the native
// control cannot be styled to sit in a row of icon buttons, and its label
// ("Choose files") says nothing about what it is for here.
const picker = ref<HTMLInputElement | null>(null);

async function onPicked(e: Event) {
  const el = e.target as HTMLInputElement;
  if (el.files?.length) await attach(el.files);
  // Cleared so picking the SAME file again still fires a change event —
  // otherwise removing a photo and re-adding it silently does nothing.
  el.value = "";
  toBottom();
}

/** Ctrl+V a screenshot straight into the field, as every chat app allows. */
async function onPaste(e: ClipboardEvent) {
  const files = Array.from(e.clipboardData?.files ?? []).filter((f) => f.type.startsWith("image/"));
  if (!files.length) return;
  e.preventDefault(); // or the browser also pastes the file name as text
  await attach(files);
  toBottom();
}

// The transcript scrolls with the PAGE rather than inside a box of its own:
// one scrollbar, the browser's own momentum and pull-to-refresh, and no inner
// region that can sit scrolled while the page is not.

/** Is the reader already at the end of the conversation? */
function atBottom(slack = 120) {
  const doc = document.documentElement;
  return doc.scrollHeight - (window.scrollY + window.innerHeight) < slack;
}

/**
 * Follow the reply as it streams — but only while the reader is already at the
 * bottom. Scrolling on every chunk regardless is what made the page lurch: read
 * back through the conversation while an answer is arriving and it dragged you
 * to the end again, several times a second.
 *
 * Whether to follow is decided BEFORE the new content is laid out, because once
 * it is rendered the page is taller and nobody is at the bottom any more.
 */
async function toBottom(force = false) {
  const follow = force || atBottom();
  await nextTick();
  if (follow) window.scrollTo({ top: document.documentElement.scrollHeight });
}
onMounted(() => toBottom(true));

/** Sending always jumps to your own message, wherever you were reading. */
function submit() {
  const done = send(() => toBottom());
  toBottom(true);
  return done;
}

const suggestions = computed(() => [t("assistant.suggest1"), t("assistant.suggest2"), t("assistant.suggest3")]);

function onKeydown(e: KeyboardEvent) {
  // Enter sends, Shift+Enter is a newline — the convention every chat uses.
  if (e.key === "Enter" && !e.shiftKey) {
    e.preventDefault();
    submit();
  }
}
</script>

<template>
  <!-- min-height, not height: the page is what scrolls. This only sets a floor,
       so a short conversation still leaves the composer at the bottom of the
       screen instead of floating halfway up it.
       svh, not dvh: dvh tracks the viewport as mobile browser chrome slides and
       the keyboard opens, so the floor would change under a sticky footer and
       shove the page around mid-scroll. svh is the stable one.
       The only thing above this page is the header: h-14 plus its 1px bottom
       border. Subtracting exactly that makes the page the height of the window,
       so an empty conversation gets no scrollbar and the composer still lands
       on the bottom edge. The page's own padding is removed for this route (see
       App.vue) rather than being subtracted here too -- a negative margin could
       not have cancelled it, because a min-height container does not shrink to
       let its last child pull upwards. -->
  <div class="mx-auto flex min-h-[calc(100svh-3.5rem-1px)] max-w-3xl flex-col pt-6 sm:pt-8">
    <!-- No page title: the nav already says which page this is, and a heading
         would only push the conversation down. Clear lives with the controls
         instead.
         flex-1 pushes the composer down while there is little to show. -->
    <div class="flex-1 space-y-5 py-4">
      <!-- Empty state: a blank box with a cursor tells nobody what to ask. -->
      <div v-if="!messages.length && !streaming" class="flex flex-col items-center gap-4 px-4 py-6 sm:py-12">
        <div class="grid size-12 place-items-center rounded-full bg-emerald-500/10 sm:size-14">
          <UIcon name="i-ui-chat" class="size-6 text-emerald-500 sm:size-7" />
        </div>
        <div class="text-center">
          <p class="text-base font-semibold">{{ t("assistant.title") }}</p>
          <p class="mt-1 text-sm text-gray-500">{{ t("assistant.intro") }}</p>
        </div>
        <div class="flex max-w-2xl flex-wrap justify-center gap-2">
          <button
            v-for="s in suggestions"
            :key="s"
            type="button"
            class="flex items-center gap-2 rounded-xl border border-default p-1 pr-3 text-left outline-none transition hover:border-emerald-500/50 hover:bg-elevated focus-visible:ring-2 focus-visible:ring-emerald-500/60"
            @click="ask(s, () => toBottom(true))"
          >
            <span class="grid size-6 shrink-0 place-items-center rounded-lg bg-emerald-500/10">
              <UIcon name="i-ui-chat" class="size-4 text-emerald-500" />
            </span>
            <span class="flex-1 truncate text-sm font-medium leading-tight">{{ s }}</span>
          </button>
        </div>
      </div>

      <template v-for="(m, i) in messages" :key="i">
        <!-- Theirs: a quiet surface, right-aligned, never the loud accent —
             the person already knows what they said. -->
        <div v-if="m.role === 'user'" class="flex flex-col items-end gap-1.5">
          <!-- Photos sit ABOVE the bubble rather than inside it: a picture is
               not a sentence, and boxing it in the same surface as the words
               makes a wall of grey. dir=rtl fills the grid from the right, so
               with one photo it lands under the bubble's right edge instead of
               stranded on the left. -->
          <div v-if="m.images?.length" dir="rtl" class="grid grid-cols-2 gap-1.5">
            <img
              v-for="(src, j) in m.images"
              :key="j"
              :src="src"
              :alt="t('assistant.photoAlt')"
              class="size-24 rounded-xl border border-muted object-cover"
            >
          </div>
          <p
            v-if="m.text"
            class="max-w-[85%] whitespace-pre-wrap rounded-xl bg-elevated px-4 py-2.5 text-sm wrap-break-word sm:text-base"
          >{{ m.text }}</p>
        </div>
        <!-- Its: no bubble, and rendered as Markdown. The answer is the page. -->
        <div v-else class="text-sm wrap-break-word sm:text-base">
          <MarkdownText :tokens="parseMarkdown(m.text)" />
        </div>
      </template>

      <!-- The reply as it arrives, in the shape it will settle into, so nothing
           jumps when it finishes. -->
      <!-- Parsed on every chunk, so formatting resolves as it is written
           rather than snapping into place at the end. -->
      <div v-if="streaming" class="text-sm wrap-break-word sm:text-base">
        <MarkdownText :tokens="parseMarkdown(streaming)" />
      </div>

      <p v-if="activity || (busy && !streaming)" class="flex items-center gap-2 text-xs text-gray-400">
        <span class="inline-block size-1.5 animate-pulse rounded-full bg-emerald-500" />
        {{ activity || t("assistant.thinking") }}
      </p>

      <p
        v-if="errorCode"
        class="flex items-start gap-2 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-200"
        role="alert"
      >
        <UIcon name="i-ui-warning" class="mt-0.5 size-4 shrink-0" />
        <span>{{ t(`errors.${errorCode}`) }}</span>
      </p>
    </div>

    <!-- Sticky, so the field stays where your thumb expects it while the
         conversation scrolls behind it.
         bg-default is the page's OWN surface token, so the bar is the same
         colour as what it sits on and reads as the bottom of the page rather
         than a slab laid over it. A hand-picked grey cannot match both themes.
         The negative margin cancels the page gutter so the bar spans the full
         width; without it, text shows through at the edges as it scrolls past. -->
    <form
      class="sticky bottom-0 z-10 -mx-4 mt-auto border-t border-default bg-default px-4 pb-[max(0.75rem,env(safe-area-inset-bottom))] pt-3 sm:-mx-6 sm:px-6"
      @submit.prevent="submit"
    >
      <!-- One box holding the field and its controls, rather than a field with
           a button parked beside it. -->
      <!-- border-muted, not border-default: on an elevated surface the default
           border token resolves to the SAME neutral as the background in dark
           mode, so both this outline and the divider below vanish. -->
      <div class="rounded-xl border border-muted bg-elevated p-2 transition focus-within:border-emerald-500/60">
        <!-- What is about to go with the message, inside the box that will send
             it — so it is plainly part of the message being written, and not a
             separate upload that happened on its own. -->
        <div v-if="attachments.length" class="flex flex-wrap gap-2 p-1 pb-2">
          <div v-for="a in attachments" :key="a.id" class="relative">
            <img :src="a.dataUrl" :alt="a.name" :title="a.name" class="size-16 rounded-lg border border-default object-cover">
            <button
              type="button"
              class="absolute -right-1.5 -top-1.5 grid size-5 place-items-center rounded-full border border-default bg-default text-dimmed outline-none transition hover:text-highlighted focus-visible:ring-2 focus-visible:ring-emerald-500/60"
              :title="t('assistant.removePhoto')"
              :aria-label="t('assistant.removePhoto')"
              @click="detach(a.id)"
            >
              <UIcon name="i-ui-close" class="size-3" />
            </button>
          </div>
        </div>

        <UTextarea
          v-model="draft"
          variant="none"
          :rows="1"
          autoresize
          :maxrows="8"
          :size="control"
          class="w-full"
          :ui="{ base: 'resize-none' }"
          :placeholder="t('assistant.placeholder')"
          @keydown="onKeydown"
          @paste="onPaste"
        />
        <div class="flex items-center gap-2 border-t border-muted pt-2">
          <!-- What it may do, next to the button that does it. Read is the
               default and the safe end: the assistant can look at the diary
               but not touch it. The server enforces this independently — the
               writing tool is not even offered in read mode — so this is a
               control, not a hint. -->
          <div class="flex items-center gap-0.5 rounded-lg bg-default p-0.5">
            <button
              v-for="m in (['read', 'write'] as const)"
              :key="m"
              type="button"
              class="rounded-md px-2 py-1 text-xs font-medium outline-none transition focus-visible:ring-2 focus-visible:ring-emerald-500/60"
              :class="mode === m
                ? 'bg-elevated text-highlighted shadow-sm'
                : 'text-dimmed hover:text-default'"
              :title="m === 'read' ? t('assistant.modeReadHint') : t('assistant.modeWriteHint')"
              :aria-pressed="mode === m"
              @click="setMode(m)"
            >{{ m === 'read' ? t("assistant.modeRead") : t("assistant.modeWrite") }}</button>
          </div>
          <!-- Icons, not words: these sit beside the send button, and three
               labels in a row would read as a toolbar rather than a control
               strip. Both keep their name for anyone who cannot see them. -->
          <input
            ref="picker"
            type="file"
            class="hidden"
            :accept="ACCEPTED_IMAGES"
            multiple
            @change="onPicked"
          >
          <UButton
            :size="inline"
            color="neutral"
            variant="ghost"
            icon="i-ui-photo"
            :disabled="busy"
            :title="t('assistant.attach')"
            :aria-label="t('assistant.attach')"
            @click="picker?.click()"
          />
          <UButton
            v-if="messages.length"
            :size="inline"
            color="neutral"
            variant="ghost"
            icon="i-ui-trash"
            :title="t('assistant.clear')"
            :aria-label="t('assistant.clear')"
            @click="clear"
          />
          <div class="flex-1" />
          <UButton
            v-if="busy"
            :size="inline"
            color="neutral"
            variant="soft"
            icon="i-ui-stop"
            :title="t('assistant.stop')"
            :aria-label="t('assistant.stop')"
            @click="stop"
          />
          <UButton
            v-else
            type="submit"
            :size="inline"
            icon="i-ui-send"
            :disabled="!canSend"
            :aria-label="t('assistant.send')"
          />
        </div>
      </div>

      <!-- Outside the field and under it: a caveat about the answers is not
           part of writing one, and inside the box it crowded the controls. -->
      <p class="mt-1.5 text-center text-[11px] text-dimmed">{{ t("assistant.disclaimer") }}</p>
    </form>
  </div>
</template>

<style scoped>
/* iOS zooms the whole page when you focus an input whose font-size is under
   16px, and does not zoom back out. Keep the compact size where there is a
   mouse, and go to 16px on touch — which suppresses the zoom without taking
   pinch-to-zoom away from anyone. Borrowed from the sibling ai-chat project,
   which hit the same thing. */
@media (pointer: coarse) {
  :deep(textarea) {
    font-size: 16px;
  }
}
</style>
