/**
 * Preparing a photo before it is sent to the assistant.
 *
 * A phone camera produces four thousand pixels across and several megabytes. A
 * vision model reads it at around a thousand and charges by the tile, so
 * sending the original wastes the person's upload, our request budget and their
 * money, and answers no better: a bowl of porridge is a bowl of porridge at
 * 1024px.
 *
 * So every picture is re-encoded here before it leaves the browser. Two things
 * fall out of that beyond the size:
 *
 *  - EXIF is dropped. Phone photos carry the GPS coordinates of where they were
 *    taken, and a meal photo is a photo of where somebody lives or works.
 *    Drawing to a canvas keeps the pixels and nothing else, so that never
 *    leaves the device. This is the reason a small photo is re-encoded too.
 *  - Orientation is baked in. createImageBitmap is asked to apply the EXIF
 *    rotation, so a portrait photo does not arrive on its side once the tag
 *    carrying that rotation has been stripped.
 */

/** What the server accepts, and what a file picker should offer. */
export const ACCEPTED_IMAGES = "image/jpeg,image/png,image/webp,image/gif";

/** The longest edge after downscaling. Above this a model sees no more detail. */
const MAX_EDGE = 1024;
/** JPEG quality. 0.82 is where a photo of food stops visibly improving. */
const QUALITY = 0.82;
/** A ceiling on what we will even open, before decoding it. */
const MAX_SOURCE_BYTES = 25 * 1024 * 1024;

export interface Attachment {
  /** Stable across a re-render, so a preview does not lose its place. */
  id: string;
  /** For the alt text and the tooltip — never sent anywhere. */
  name: string;
  /** `data:image/jpeg;base64,…` — shown directly in an <img>. */
  dataUrl: string;
}

export class ImageError extends Error {
  constructor(readonly code: "image_type" | "image_too_large" | "image_unreadable") {
    super(code);
  }
}

let counter = 0;

/**
 * Read one picked file into something that can be previewed and sent.
 *
 * Throws an ImageError whose `code` is an i18n key, so the caller can say what
 * went wrong in the person's own language without inspecting the message.
 */
export async function prepareImage(file: File): Promise<Attachment> {
  if (!ACCEPTED_IMAGES.split(",").includes(file.type)) throw new ImageError("image_type");
  if (file.size > MAX_SOURCE_BYTES) throw new ImageError("image_too_large");

  let bitmap: ImageBitmap;
  try {
    // imageOrientation is what applies the EXIF rotation; without it the tag is
    // stripped along with everything else and the photo arrives sideways.
    bitmap = await createImageBitmap(file, { imageOrientation: "from-image" });
  } catch {
    throw new ImageError("image_unreadable"); // not really an image, or a codec this browser lacks
  }

  try {
    const scale = Math.min(1, MAX_EDGE / Math.max(bitmap.width, bitmap.height));
    const canvas = document.createElement("canvas");
    canvas.width = Math.max(1, Math.round(bitmap.width * scale));
    canvas.height = Math.max(1, Math.round(bitmap.height * scale));

    const ctx = canvas.getContext("2d");
    if (!ctx) throw new ImageError("image_unreadable");
    // A transparent PNG would otherwise flatten onto black, which turns the
    // background of a cropped photo into a hole.
    ctx.fillStyle = "#ffffff";
    ctx.fillRect(0, 0, canvas.width, canvas.height);
    ctx.drawImage(bitmap, 0, 0, canvas.width, canvas.height);

    // JPEG for everything: a photograph is what this is for, and PNG of a
    // photograph is several times the size for no visible gain.
    return {
      id: `img-${Date.now()}-${counter++}`,
      name: file.name || "photo.jpg",
      dataUrl: canvas.toDataURL("image/jpeg", QUALITY),
    };
  } finally {
    bitmap.close(); // the decoded bitmap is the big one; do not wait for the GC
  }
}

/**
 * Split a data URL into the two fields the API wants.
 *
 * The wire format is the media type and bare base64, because that is the shape
 * every vision API takes — and it means the server never has to parse a URL to
 * find where the payload starts.
 */
export function toWire(dataUrl: string): { mediaType: string; data: string } | null {
  const match = /^data:([^;,]+);base64,(.*)$/s.exec(dataUrl);
  return match ? { mediaType: match[1], data: match[2] } : null;
}
