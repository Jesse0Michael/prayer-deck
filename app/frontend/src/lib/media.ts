import type { DealtCard, Media } from "../api/types";

/** The media a card shows: the face it was dealt with, otherwise its newest. */
export function faceOf(card: DealtCard): Media | null {
  return card.face ?? card.media[0] ?? null;
}

const decoded = new Map<string, Promise<void>>();

/** Loads and decodes an image once, so revealing a card never shows a half-painted face. */
export function preloadImage(url: string): Promise<void> {
  let pending = decoded.get(url);
  if (!pending) {
    const img = new Image();
    img.decoding = "async";
    img.src = url;
    pending = img.decode().catch(() => {
      decoded.delete(url);
    });
    decoded.set(url, pending);
  }
  return pending;
}

/** Preloads every card face, giving up after timeoutMs so a slow network never blocks the deal. */
export async function preloadFaces(cards: DealtCard[], timeoutMs = 6000): Promise<void> {
  const loads = cards.flatMap((c) => {
    const face = faceOf(c);
    return face ? [preloadImage(face.display_url)] : [];
  });
  await Promise.race([Promise.all(loads), new Promise((r) => setTimeout(r, timeoutMs))]);
}
