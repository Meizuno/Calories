import { Lexer, type Token, type MarkedOptions } from "marked";

// Parsing only — never rendering. marked can emit HTML and we deliberately do
// not use that path: what comes back here is a token tree that
// components/MarkdownText.vue turns into Vue components, so no model output is
// ever interpreted as markup.
const options: MarkedOptions = {
  // GitHub-flavoured: tables and ~~strikethrough~~, which a model reaches for
  // when it lists numbers.
  gfm: true,
  // A single newline is a line break. Models write lists that way and expect
  // them to stay on separate lines.
  breaks: true,
};

/**
 * Parse Markdown into tokens.
 *
 * Lexer.lex is the STATIC method, which builds a fresh lexer per call. A shared
 * instance cannot be reused: its lex() appends to an array the constructor
 * created and never clears, so parsing the same growing string once per
 * streamed chunk appends another copy of the whole reply each time — the
 * message visibly repeats, and the work grows with the square of its length.
 *
 * Never throws: a half-written reply arrives here on every chunk, and an
 * unclosed table or a stray backtick must not blank the message. Anything
 * unparseable falls back to one plain paragraph.
 */
export function parseMarkdown(src: string): Token[] {
  try {
    return Lexer.lex(src, options);
  } catch {
    return [
      { type: "paragraph", raw: src, text: src, tokens: [{ type: "text", raw: src, text: src }] } as Token,
    ];
  }
}
