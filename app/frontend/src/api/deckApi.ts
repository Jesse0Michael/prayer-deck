import type { Card, CardInput, Deck, Draw, Note } from "./types";

/** The deck, card, note, and draw operations behind the signed-in views. */
export interface DeckApi {
  /** The account's decks; the views use the first. */
  decks(): Promise<Deck[]>;
  /** Every card in the deck, sorted by name. */
  cards(deckID: string): Promise<Card[]>;
  createCard(deckID: string, input: CardInput): Promise<Card>;
  updateCard(cardID: string, input: CardInput): Promise<Card>;
  deleteCard(cardID: string): Promise<void>;
  /** One page of a card's notes, newest first, with the card's total note count. */
  notes(cardID: string, limit: number, offset: number): Promise<{ notes: Note[]; total: number }>;
  createNote(cardID: string, text: string): Promise<Note>;
  updateNote(noteID: string, text: string): Promise<Note>;
  deleteNote(noteID: string): Promise<void>;
  /** Adds an image as the card's newest media, which becomes its primary face. */
  addMedia(cardID: string, file: Blob): Promise<Card>;
  deleteMedia(cardID: string, mediaID: string): Promise<void>;
  /** Deals the next cards in the pass; with shuffle, every card returns to the pass first. */
  draw(deckID: string, count: number, shuffle: boolean): Promise<Draw>;
  /** The deck's most recent draw, or null when nothing has been drawn yet. */
  currentDraw(deckID: string): Promise<Draw | null>;
}
