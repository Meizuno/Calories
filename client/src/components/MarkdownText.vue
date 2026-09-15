<script setup lang="ts">
import type { Token, Tokens } from "marked";
import ProseA from "./prose/ProseA.vue";
import ProseBlockquote from "./prose/ProseBlockquote.vue";
import ProseCode from "./prose/ProseCode.vue";
import ProseDel from "./prose/ProseDel.vue";
import ProseEm from "./prose/ProseEm.vue";
import ProseHeading from "./prose/ProseHeading.vue";
import ProseHr from "./prose/ProseHr.vue";
import ProseLi from "./prose/ProseLi.vue";
import ProseOl from "./prose/ProseOl.vue";
import ProseP from "./prose/ProseP.vue";
import ProsePre from "./prose/ProsePre.vue";
import ProseStrong from "./prose/ProseStrong.vue";
import ProseTable from "./prose/ProseTable.vue";
import ProseUl from "./prose/ProseUl.vue";

// Renders a marked token tree as Vue components.
//
// Token tree rather than HTML, and components rather than v-html, because this
// renders MODEL output — which can carry whatever a tool result carried. There
// is no HTML string anywhere in this path, so there is nothing for a crafted
// reply to inject: an element either has a component here or it is not drawn.
// `html` tokens are deliberately ignored for the same reason.
//
// The component recurses into itself for nested content (a list inside a list,
// bold inside a heading). Vue resolves a <script setup> component by its own
// filename, so <MarkdownText> below is this file.
defineProps<{ tokens: Token[] }>();

const isList = (t: Token): t is Tokens.List => t.type === "list";
const isTable = (t: Token): t is Tokens.Table => t.type === "table";
const isHeading = (t: Token): t is Tokens.Heading => t.type === "heading";
const isLink = (t: Token): t is Tokens.Link => t.type === "link";
const isCode = (t: Token): t is Tokens.Code => t.type === "code";
const isCodespan = (t: Token): t is Tokens.Codespan => t.type === "codespan";

/** Inline children, or nothing — several token types carry both. */
const kids = (t: Token): Token[] => ("tokens" in t && t.tokens ? (t.tokens as Token[]) : []);
/** The literal text of a leaf token. */
const raw = (t: Token): string => ("text" in t ? String((t as { text: unknown }).text) : "");
</script>

<template>
  <template v-for="(tok, i) in tokens" :key="i">
    <ProseP v-if="tok.type === 'paragraph'">
      <MarkdownText :tokens="kids(tok)" />
    </ProseP>

    <ProseHeading v-else-if="isHeading(tok)" :depth="tok.depth">
      <MarkdownText :tokens="kids(tok)" />
    </ProseHeading>

    <component :is="isList(tok) && tok.ordered ? ProseOl : ProseUl" v-else-if="isList(tok)">
      <ProseLi v-for="(item, j) in tok.items" :key="j">
        <MarkdownText :tokens="item.tokens ?? []" />
      </ProseLi>
    </component>

    <ProseBlockquote v-else-if="tok.type === 'blockquote'">
      <MarkdownText :tokens="kids(tok)" />
    </ProseBlockquote>

    <ProsePre v-else-if="isCode(tok)">{{ tok.text }}</ProsePre>

    <ProseTable
      v-else-if="isTable(tok)"
      :header="tok.header"
      :rows="tok.rows"
      :align="tok.align"
    >
      <template #cell="{ tokens: cellTokens }">
        <MarkdownText :tokens="cellTokens" />
      </template>
    </ProseTable>

    <ProseHr v-else-if="tok.type === 'hr'" />

    <!-- inline -->
    <ProseStrong v-else-if="tok.type === 'strong'">
      <MarkdownText :tokens="kids(tok)" />
    </ProseStrong>
    <ProseEm v-else-if="tok.type === 'em'">
      <MarkdownText :tokens="kids(tok)" />
    </ProseEm>
    <ProseDel v-else-if="tok.type === 'del'">
      <MarkdownText :tokens="kids(tok)" />
    </ProseDel>
    <ProseCode v-else-if="isCodespan(tok)">{{ tok.text }}</ProseCode>
    <ProseA v-else-if="isLink(tok)" :href="tok.href" :title="tok.title ?? undefined">
      <MarkdownText :tokens="kids(tok)" />
    </ProseA>
    <br v-else-if="tok.type === 'br'" />

    <!-- A text token may still have inline children (bold inside a list item). -->
    <template v-else-if="tok.type === 'text'">
      <MarkdownText v-if="kids(tok).length" :tokens="kids(tok)" />
      <template v-else>{{ raw(tok) }}</template>
    </template>

    <!-- Anything else — raw html, footnotes, whatever a model invents — is
         dropped rather than guessed at. `space` is genuinely nothing. -->
  </template>
</template>
