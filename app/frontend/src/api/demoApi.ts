import { ApiError } from "./client";
import type { DeckApi } from "./deckApi";
import type { Card, CardInput, Deck, Draw, DrawnCard, Media, Note } from "./types";

const DEMO_DECK_ID = "demo-deck";
const DAY_MS = 86_400_000;
const LATENCY_MS = 120;

interface Seed {
  name: string;
  /** Newest first, each with how many days ago it was written. */
  notes: [daysAgo: number, text: string][];
}

const SEEDS: Seed[] = [
  {
    name: "Grandma Ruth",
    notes: [
      [1, "Hip surgery went well. Pray for a steady recovery and good company in rehab."],
      [9, "Surgery scheduled for the 14th. She is nervous but trying not to show it."],
    ],
  },
  { name: "Sam & Joy", notes: [[3, "Expecting in March. Pray for rest, and for peace about the move."]] },
  { name: "Uncle Dale", notes: [[6, "New job with a long commute. Pray for patience and for evenings at home."]] },
  {
    name: "The Okafors",
    notes: [
      [0, "Chidi had a hard first week. Pray for one good friend."],
      [12, "Chidi starts kindergarten. Pray for brave mornings."],
    ],
  },
  { name: "Marcus", notes: [[2, "Finals all next week. Pray for focus, and for sleep."]] },
  { name: "Aunt Lena", notes: [[20, "Lonely since the move. Pray for a church family close by."]] },
  { name: "Pastor Ben", notes: [] },
  { name: "Priya", notes: [[4, "Waiting on test results. Pray for calm while she waits."]] },
  { name: "Mom & Dad", notes: [[8, "Their 40th anniversary is coming up. Give thanks for them."]] },
];

interface DemoCard {
  card: Card;
  /** Newest first. */
  notes: Note[];
  lastDrawnAt: number | null;
}

interface DemoDraw {
  id: string;
  cardIDs: string[];
  faces: Record<string, string>;
  createdAt: number;
}

/**
 * An in-memory deck of made-up people that answers like the API, so the
 * landing page can run the real views without an account or a server.
 */
export class DemoApi implements DeckApi {
  private cardsByID = new Map<string, DemoCard>();
  private shuffledAt: number;
  private current: DemoDraw | null = null;
  private lastTime = 0;
  private nextID = 0;
  /** Keeps IDs from one demo apart from a reset one's, since the views remember draws by ID. */
  private readonly instance = Date.now().toString(36);
  private readonly createdAt: number;

  constructor() {
    const now = Date.now();
    this.createdAt = now - 30 * DAY_MS;
    this.shuffledAt = this.createdAt;
    SEEDS.forEach((seed, i) => {
      const created = new Date(this.createdAt + i * 60_000).toISOString();
      const id = this.id("card");
      const notes = seed.notes.map(([daysAgo, text]) => {
        const at = new Date(now - daysAgo * DAY_MS - 3_600_000).toISOString();
        return { id: this.id("note"), text, created_at: at, updated_at: at };
      });
      this.cardsByID.set(id, {
        card: { id, name: seed.name, media: [], created_at: created, updated_at: created },
        notes,
        lastDrawnAt: null,
      });
    });
  }

  async decks(): Promise<Deck[]> {
    const at = new Date(this.createdAt).toISOString();
    return respond([
      {
        id: DEMO_DECK_ID,
        name: "My Deck",
        card_count: this.cardsByID.size,
        remaining: this.pool().length,
        created_at: at,
        updated_at: at,
      },
    ]);
  }

  async cards(): Promise<Card[]> {
    const cards = [...this.cardsByID.values()].map((c) => c.card).sort((a, b) => a.name.localeCompare(b.name));
    return respond(cards);
  }

  async createCard(_deckID: string, input: CardInput): Promise<Card> {
    const at = new Date(this.now()).toISOString();
    const card: Card = { id: this.id("card"), name: input.name ?? "", media: [], created_at: at, updated_at: at };
    this.cardsByID.set(card.id, { card, notes: [], lastDrawnAt: null });
    return respond(card);
  }

  async updateCard(cardID: string, input: CardInput): Promise<Card> {
    const entry = this.entry(cardID);
    if (input.name !== undefined) entry.card.name = input.name;
    entry.card.updated_at = new Date(this.now()).toISOString();
    return respond(entry.card);
  }

  async deleteCard(cardID: string): Promise<void> {
    this.entry(cardID);
    this.cardsByID.delete(cardID);
    return respond(undefined);
  }

  async notes(cardID: string, limit: number, offset: number): Promise<{ notes: Note[]; total: number }> {
    const { notes } = this.entry(cardID);
    return respond({ notes: notes.slice(offset, offset + limit), total: notes.length });
  }

  async createNote(cardID: string, text: string): Promise<Note> {
    const at = new Date(this.now()).toISOString();
    const note: Note = { id: this.id("note"), text, created_at: at, updated_at: at };
    this.entry(cardID).notes.unshift(note);
    return respond(note);
  }

  async updateNote(noteID: string, text: string): Promise<Note> {
    const note = this.note(noteID);
    note.text = text;
    note.updated_at = new Date(this.now()).toISOString();
    return respond(note);
  }

  async deleteNote(noteID: string): Promise<void> {
    for (const entry of this.cardsByID.values()) {
      entry.notes = entry.notes.filter((n) => n.id !== noteID);
    }
    return respond(undefined);
  }

  async addMedia(cardID: string, file: Blob): Promise<Card> {
    const entry = this.entry(cardID);
    const url = URL.createObjectURL(file);
    let width = 0;
    let height = 0;
    try {
      const bitmap = await createImageBitmap(file);
      ({ width, height } = bitmap);
      bitmap.close();
    } catch {
      throw new ApiError(400, "That file isn't an image we can read");
    }
    const media: Media = {
      id: this.id("media"),
      kind: "image",
      display_url: url,
      thumbnail_url: url,
      original_url: url,
      width,
      height,
      created_at: new Date(this.now()).toISOString(),
    };
    entry.card.media.unshift(media);
    return respond(entry.card);
  }

  async deleteMedia(cardID: string, mediaID: string): Promise<void> {
    const entry = this.entry(cardID);
    entry.card.media = entry.card.media.filter((m) => m.id !== mediaID);
    return respond(undefined);
  }

  async draw(_deckID: string, count: number, shuffle: boolean): Promise<Draw> {
    if (this.cardsByID.size === 0) throw new ApiError(422, "deck has no cards");
    const now = this.now();
    if (shuffle) this.shuffledAt = now;

    const pool = this.pool();
    if (pool.length === 0) throw new ApiError(409, "every card has been drawn; shuffle to start again");
    for (let i = pool.length - 1; i > 0; i--) {
      const j = Math.floor(Math.random() * (i + 1));
      [pool[i], pool[j]] = [pool[j], pool[i]];
    }

    const hand = pool.slice(0, count);
    const faces: Record<string, string> = {};
    for (const entry of hand) {
      entry.lastDrawnAt = now;
      const { media } = entry.card;
      if (media.length > 0) faces[entry.card.id] = media[Math.floor(Math.random() * media.length)].id;
    }
    this.current = { id: this.id("draw"), cardIDs: hand.map((e) => e.card.id), faces, createdAt: now };
    return respond(this.drawView(this.current));
  }

  async currentDraw(): Promise<Draw | null> {
    return respond(this.current ? this.drawView(this.current) : null);
  }

  private drawView(d: DemoDraw): Draw {
    const cards: DrawnCard[] = d.cardIDs.flatMap((id) => {
      const entry = this.cardsByID.get(id);
      if (!entry) return [];
      const face = entry.card.media.find((m) => m.id === d.faces[id]) ?? null;
      return [{ ...entry.card, face }];
    });
    const discard = [...this.cardsByID.values()]
      .filter((e) => e.lastDrawnAt !== null && e.lastDrawnAt >= this.shuffledAt && !d.cardIDs.includes(e.card.id))
      .sort((a, b) => (a.lastDrawnAt ?? 0) - (b.lastDrawnAt ?? 0))
      .map((e) => e.card);
    return {
      id: d.id,
      cards,
      discard,
      remaining: this.pool().length,
      created_at: new Date(d.createdAt).toISOString(),
    };
  }

  /** Cards still to be dealt in the pass that began at shuffledAt. */
  private pool(): DemoCard[] {
    return [...this.cardsByID.values()].filter((e) => e.lastDrawnAt === null || e.lastDrawnAt < this.shuffledAt);
  }

  private entry(cardID: string): DemoCard {
    const entry = this.cardsByID.get(cardID);
    if (!entry) throw new ApiError(404, "card not found");
    return entry;
  }

  private note(noteID: string): Note {
    for (const entry of this.cardsByID.values()) {
      const note = entry.notes.find((n) => n.id === noteID);
      if (note) return note;
    }
    throw new ApiError(404, "note not found");
  }

  /** Strictly increasing, so a shuffle and the draw after it never share a timestamp. */
  private now(): number {
    this.lastTime = Math.max(Date.now(), this.lastTime + 1);
    return this.lastTime;
  }

  private id(kind: string): string {
    this.nextID += 1;
    return `demo-${this.instance}-${kind}-${this.nextID}`;
  }
}

/** Copies the result so views never share state with the demo store, after a pause like a real request. */
async function respond<T>(value: T): Promise<T> {
  await new Promise((r) => setTimeout(r, LATENCY_MS));
  return value === undefined ? value : structuredClone(value);
}
