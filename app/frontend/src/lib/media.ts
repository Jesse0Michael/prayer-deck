import type { Card } from "../api/types";

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
export async function preloadFaces(cards: Card[], timeoutMs = 6000): Promise<void> {
  const loads = cards.flatMap((c) => (c.artwork ? [preloadImage(c.artwork.display_url)] : []));
  await Promise.race([Promise.all(loads), new Promise((r) => setTimeout(r, timeoutMs))]);
}
