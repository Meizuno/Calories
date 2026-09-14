/**
 * Unwrap a standalone SVG file into the `body` Iconify expects.
 *
 * The icons and flags live in `src/assets` as complete `.svg` files, so they
 * open in any editor or preview and can be swapped without touching code.
 * Iconify wants only what is *inside* the root element — everything else comes
 * from the collection's `width`/`height` — so the wrapper is peeled off here,
 * once, at build time: the files are inlined by Vite's `?raw`, and this runs on
 * a string constant rather than a network response.
 */
export function svgBody(svg: string): string {
  return svg
    .replace(/^[\s\S]*?<svg\b[^>]*>/, "")
    .replace(/<\/svg>[\s\S]*$/, "")
    .trim();
}
