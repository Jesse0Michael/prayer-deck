import type { ApiClient } from "./client";
import type { DeckApi } from "./deckApi";
import type { Card, CardInput, Deck, Draw, DrawnCard, Media, Note, User } from "./types";

const CACHE_KEY = "prayer-deck.cache";
/** Cached media URLs this close to expiring count as already expired. */
const URL_EXPIRY_MARGIN_MS = 10 * 60_000;

/** A card's notes from newest, contiguous from the first. */
interface NoteList {
  notes: Note[];
  total: number;
}

interface CacheData {
  me?: User;
  decks?: Deck[];
  cards: Record<string, Card[]>;
  draws: Record<string, Draw | null>;
  notes: Record<string, NoteList>;
}

const empty = (): CacheData => ({ cards: {}, draws: {}, notes: {} });

/**
 * A DeckApi that answers reads from local storage and goes to the server only
 * for writes, cache misses, expiring media URLs, and a shuffle. The server
 * cold-starts and bills per request, so a visit that only reads costs nothing.
 * Changes made on another device appear after the next shuffle.
 */
export class CachedDeckApi implements DeckApi {
  private data: CacheData;

  constructor(private readonly client: ApiClient) {
    this.data = load();
    client.onAuthChange((signedIn) => {
      if (!signedIn) this.clear();
    });
  }

  /** The signed-in user. */
  async me(): Promise<User> {
    if (!this.data.me) {
      this.data.me = await this.client.me();
      this.save();
    }
    return structuredClone(this.data.me);
  }

  async decks(): Promise<Deck[]> {
    if (!this.data.decks) {
      this.data.decks = await this.client.decks();
      this.save();
    }
    return structuredClone(this.data.decks);
  }

  async cards(deckID: string): Promise<Card[]> {
    const cached = this.data.cards[deckID];
    if (cached && !expiring(cached)) return structuredClone(cached);
    const cards = await this.client.cards(deckID);
    this.data.cards[deckID] = structuredClone(cards);
    this.save();
    return cards;
  }

  async currentDraw(deckID: string): Promise<Draw | null> {
    const cached = this.data.draws[deckID];
    if (cached === null) return null;
    if (cached && !expiring([...cached.cards, ...cached.discard])) return structuredClone(cached);
    const draw = await this.client.currentDraw(deckID);
    this.data.draws[deckID] = structuredClone(draw);
    this.save();
    return draw;
  }

  async draw(deckID: string, count: number, shuffle: boolean): Promise<Draw> {
    const dealt = await this.client.draw(deckID, count, shuffle);
    this.data.draws[deckID] = structuredClone(dealt);
    this.save();
    if (shuffle) void this.refresh(deckID);
    return dealt;
  }

  async createCard(deckID: string, input: CardInput): Promise<Card> {
    const card = await this.client.createCard(deckID, input);
    const cards = this.data.cards[deckID];
    if (cards) this.data.cards[deckID] = sortByName([...cards, structuredClone(card)]);
    // A new card has never been drawn, so it joins the current pass.
    const draw = this.data.draws[deckID];
    if (draw) draw.remaining += 1;
    this.save();
    return card;
  }

  async updateCard(cardID: string, input: CardInput): Promise<Card> {
    const card = await this.client.updateCard(cardID, input);
    this.replaceCard(card);
    return card;
  }

  async deleteCard(cardID: string): Promise<void> {
    await this.client.deleteCard(cardID);
    for (const [deckID, cards] of Object.entries(this.data.cards)) {
      this.data.cards[deckID] = cards.filter((c) => c.id !== cardID);
    }
    for (const draw of Object.values(this.data.draws)) {
      if (!draw) continue;
      const inHand = draw.cards.some((c) => c.id === cardID);
      const inDiscard = draw.discard.some((c) => c.id === cardID);
      if (!inHand && !inDiscard) draw.remaining = Math.max(draw.remaining - 1, 0);
      draw.cards = draw.cards.filter((c) => c.id !== cardID);
      draw.discard = draw.discard.filter((c) => c.id !== cardID);
    }
    delete this.data.notes[cardID];
    this.save();
  }

  async addMedia(cardID: string, file: Blob): Promise<Card> {
    const card = await this.client.addMedia(cardID, file);
    this.replaceCard(card);
    return card;
  }

  async deleteMedia(cardID: string, mediaID: string): Promise<void> {
    await this.client.deleteMedia(cardID, mediaID);
    const withoutMedia = <T extends Card>(card: T): T =>
      card.id === cardID ? { ...card, media: card.media.filter((m) => m.id !== mediaID) } : card;
    for (const [deckID, cards] of Object.entries(this.data.cards)) {
      this.data.cards[deckID] = cards.map(withoutMedia);
    }
    for (const draw of Object.values(this.data.draws)) {
      if (!draw) continue;
      draw.cards = draw.cards.map((c) => {
        const card = withoutMedia(c);
        return card.face?.id === mediaID ? { ...card, face: null } : card;
      });
      draw.discard = draw.discard.map(withoutMedia);
    }
    this.save();
  }

  async notes(cardID: string, limit: number, offset: number): Promise<{ notes: Note[]; total: number }> {
    const cached = this.data.notes[cardID];
    if (cached && (offset + limit <= cached.notes.length || cached.notes.length >= cached.total)) {
      return structuredClone({ notes: cached.notes.slice(offset, offset + limit), total: cached.total });
    }
    const page = await this.client.notes(cardID, limit, offset);
    const known = cached?.notes ?? [];
    // Only a page that continues the cached run can be appended without leaving a gap.
    if (offset <= known.length) {
      this.data.notes[cardID] = structuredClone({ notes: [...known.slice(0, offset), ...page.notes], total: page.total });
      this.save();
    }
    return page;
  }

  async createNote(cardID: string, text: string): Promise<Note> {
    const note = await this.client.createNote(cardID, text);
    const cached = this.data.notes[cardID];
    if (cached) {
      this.data.notes[cardID] = { notes: [structuredClone(note), ...cached.notes], total: cached.total + 1 };
      this.save();
    }
    return note;
  }

  async updateNote(noteID: string, text: string): Promise<Note> {
    const note = await this.client.updateNote(noteID, text);
    for (const list of Object.values(this.data.notes)) {
      list.notes = list.notes.map((n) => (n.id === noteID ? structuredClone(note) : n));
    }
    this.save();
    return note;
  }

  async deleteNote(noteID: string): Promise<void> {
    await this.client.deleteNote(noteID);
    for (const list of Object.values(this.data.notes)) {
      if (list.notes.some((n) => n.id === noteID)) {
        list.notes = list.notes.filter((n) => n.id !== noteID);
        list.total -= 1;
      }
    }
    this.save();
  }

  /** Replaces the deck and its cards with the server's, and drops notes so they reload when opened. */
  private async refresh(deckID: string): Promise<void> {
    try {
      const [decks, cards] = await Promise.all([this.client.decks(), this.client.cards(deckID)]);
      this.data.decks = decks;
      this.data.cards[deckID] = cards;
      this.data.notes = {};
      this.save();
    } catch (err) {
      console.warn("failed to refresh the cached deck", err);
    }
  }

  private replaceCard(updated: Card): void {
    const card = structuredClone(updated);
    for (const [deckID, cards] of Object.entries(this.data.cards)) {
      if (cards.some((c) => c.id === card.id)) {
        this.data.cards[deckID] = sortByName(cards.map((c) => (c.id === card.id ? card : c)));
      }
    }
    for (const draw of Object.values(this.data.draws)) {
      if (!draw) continue;
      draw.cards = draw.cards.map((c): DrawnCard => (c.id === card.id ? { ...card, face: keptFace(c, card) } : c));
      draw.discard = draw.discard.map((c) => (c.id === card.id ? card : c));
    }
    this.save();
  }

  private clear(): void {
    this.data = empty();
    localStorage.removeItem(CACHE_KEY);
  }

  private save(): void {
    try {
      localStorage.setItem(CACHE_KEY, JSON.stringify(this.data));
    } catch (err) {
      console.warn("failed to save the deck cache", err);
    }
  }
}

function load(): CacheData {
  try {
    const stored = JSON.parse(localStorage.getItem(CACHE_KEY) ?? "null") as CacheData | null;
    if (stored) return { ...empty(), ...stored };
  } catch {
    // A corrupt cache is rebuilt from the server.
  }
  return empty();
}

function sortByName(cards: Card[]): Card[] {
  return [...cards].sort((a, b) => a.name.localeCompare(b.name));
}

/** The dealt face with fresh URLs from the updated card, or null once that media is gone. */
function keptFace(dealt: DrawnCard, card: Card): Media | null {
  if (!dealt.face) return null;
  return card.media.find((m) => m.id === dealt.face?.id) ?? null;
}

/** Whether any media URL on these cards expires within the margin. */
function expiring(cards: (Card | DrawnCard)[]): boolean {
  const deadline = Date.now() + URL_EXPIRY_MARGIN_MS;
  return cards.some((card) => {
    const media = "face" in card && card.face ? [card.face, ...card.media] : card.media;
    return media.some((m) => {
      const expires = urlExpiry(m.display_url);
      return expires !== null && expires < deadline;
    });
  });
}

/** When a presigned S3 URL stops working, in epoch ms; null for URLs that never expire. */
function urlExpiry(url: string): number | null {
  let params: URLSearchParams;
  try {
    params = new URL(url).searchParams;
  } catch {
    return null;
  }
  const date = params.get("X-Amz-Date");
  const seconds = Number(params.get("X-Amz-Expires"));
  const match = date?.match(/^(\d{4})(\d{2})(\d{2})T(\d{2})(\d{2})(\d{2})Z$/);
  if (!match || !seconds) return null;
  const [, y, mo, d, h, mi, s] = match.map(Number);
  return Date.UTC(y, mo - 1, d, h, mi, s) + seconds * 1000;
}
