import amandaThumbnail from "../assets/demo/amanda-thumb.jpg";
import amandaDisplay from "../assets/demo/amanda.jpg";
import clementineThumbnail from "../assets/demo/clementine-thumb.jpg";
import clementineDisplay from "../assets/demo/clementine.jpg";
import dadThumbnail from "../assets/demo/dad-thumb.jpg";
import dadDisplay from "../assets/demo/dad.jpg";
import foothillsThumbnail from "../assets/demo/foothills_baptist_church-thumb.jpg";
import foothillsDisplay from "../assets/demo/foothills_baptist_church.jpg";
import grandmaThumbnail from "../assets/demo/grandma-thumb.jpg";
import grandmaDisplay from "../assets/demo/grandma.jpg";
import jacobMollyThumbnail from "../assets/demo/jacob_molly-thumb.jpg";
import jacobMollyDisplay from "../assets/demo/jacob_molly.jpg";
import keatanThumbnail from "../assets/demo/keatan-thumb.jpg";
import keatanDisplay from "../assets/demo/keatan.jpg";
import lorelaiThumbnail from "../assets/demo/lorelai-thumb.jpg";
import lorelaiDisplay from "../assets/demo/lorelai.jpg";
import mccoysThumbnail from "../assets/demo/mccoys-thumb.jpg";
import mccoysDisplay from "../assets/demo/mccoys.jpg";
import { ApiError } from "./client";
import type { DeckApi } from "./deckApi";
import type { Card, CardInput, Deck, Draw, DrawnCard, Media, Note } from "./types";

const DEMO_DECK_ID = "demo-deck";
const DAY_MS = 86_400_000;
const LATENCY_MS = 120;

/** Every bundled demo image shares these dimensions. */
const IMAGE_WIDTH = 805;
const IMAGE_HEIGHT = 1200;

interface Seed {
  name: string;
  /** Bundled artwork URLs. */
  image: { display: string; thumbnail: string };
  /** Newest first, each with how many days ago it was written. */
  notes: [daysAgo: number, text: string][];
}

const SEEDS: Seed[] = [
  { name: "Amanda", image: { display: amandaDisplay, thumbnail: amandaThumbnail }, notes: [] },
  { name: "Clementine", image: { display: clementineDisplay, thumbnail: clementineThumbnail }, notes: [] },
  {
    name: "Dad",
    image: { display: dadDisplay, thumbnail: dadThumbnail },
    notes: [[6, "Pray for strength and good health this season."]],
  },
  {
    name: "Foothills Baptist Church",
    image: { display: foothillsDisplay, thumbnail: foothillsThumbnail },
    notes: [
      [1, "Pray for Sunday's service and everyone who walks through the doors."],
      [10, "Give thanks for the volunteers who set up every week."],
    ],
  },
  {
    name: "Grandma",
    image: { display: grandmaDisplay, thumbnail: grandmaThumbnail },
    notes: [[2, "Give thanks for her, and pray for good days this week."]],
  },
  {
    name: "Jacob & Molly",
    image: { display: jacobMollyDisplay, thumbnail: jacobMollyThumbnail },
    notes: [[3, "Pray for their marriage, and for rest."]],
  },
  {
    name: "Keatan",
    image: { display: keatanDisplay, thumbnail: keatanThumbnail },
    notes: [[4, "Pray for a good week at school."]],
  },
  { name: "Lorelai", image: { display: lorelaiDisplay, thumbnail: lorelaiThumbnail }, notes: [] },
  { name: "The McCoys", image: { display: mccoysDisplay, thumbnail: mccoysThumbnail }, notes: [] },
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
 * An in-memory sample deck that answers like the API, so the
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
      const media: Media = {
        id: this.id("media"),
        kind: "image",
        display_url: seed.image.display,
        thumbnail_url: seed.image.thumbnail,
        original_url: seed.image.display,
        width: IMAGE_WIDTH,
        height: IMAGE_HEIGHT,
        created_at: created,
      };
      this.cardsByID.set(id, {
        card: { id, name: seed.name, media: [media], created_at: created, updated_at: created },
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
