import { AnimatePresence, motion } from "motion/react";
import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import type { ApiClient } from "../api/client";
import type { Card, Deck } from "../api/types";
import { CardFace } from "../components/CardFace";
import { CardEditor } from "./CardEditor";

interface ManageViewProps {
  api: ApiClient;
  deck: Deck;
  decks: Deck[];
  onDecksChanged: (selectDeckID?: string) => void;
}

type Editing = { card?: Card } | null;

export function ManageView({ api, deck, decks, onDecksChanged }: ManageViewProps) {
  const [cards, setCards] = useState<Card[] | null>(null);
  const [editing, setEditing] = useState<Editing>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setCards(null);
    api
      .cards(deck.id)
      .then((c) => !cancelled && setCards(c))
      .catch((err: Error) => !cancelled && setError(err.message));
    return () => {
      cancelled = true;
    };
  }, [api, deck.id]);

  const sortCards = (list: Card[]) => [...list].sort((a, b) => a.name.localeCompare(b.name));

  const onSaved = (saved: Card) => {
    setCards((prev) => sortCards([...(prev ?? []).filter((c) => c.id !== saved.id), saved]));
    setEditing(null);
    onDecksChanged();
  };

  const onDeleted = (cardID: string) => {
    setCards((prev) => (prev ?? []).filter((c) => c.id !== cardID));
    setEditing(null);
    onDecksChanged();
  };

  return (
    <div className="manage-view">
      <DeckToolbar api={api} deck={deck} decks={decks} onDecksChanged={onDecksChanged} />

      {error && <p className="error-text">{error}</p>}

      <motion.ul className="card-grid" layout>
        <li>
          <button type="button" className="grid-card grid-add" onClick={() => setEditing({})}>
            <span className="grid-add-plus">+</span>
            <span>Add a card</span>
          </button>
        </li>
        <AnimatePresence initial={false}>
          {cards?.map((card) => (
            <motion.li
              key={card.id}
              layout
              initial={{ opacity: 0, scale: 0.9 }}
              animate={{ opacity: 1, scale: 1 }}
              exit={{ opacity: 0, scale: 0.9 }}
              transition={{ type: "spring", stiffness: 260, damping: 24 }}
            >
              <motion.button
                type="button"
                className="grid-card"
                onClick={() => setEditing({ card })}
                whileHover={{ y: -6, rotate: -0.6 }}
                whileTap={{ scale: 0.97 }}
              >
                <CardFace card={card} thumbnail />
                <span className="grid-card-name">{card.name}</span>
              </motion.button>
            </motion.li>
          ))}
        </AnimatePresence>
      </motion.ul>

      {cards?.length === 0 && (
        <p className="empty-note">Start with the people who come to mind first. You can add artwork now or later.</p>
      )}

      {createPortal(
        <AnimatePresence>
          {editing && (
          <CardEditor
            api={api}
            deckID={deck.id}
            card={editing.card}
            onSaved={onSaved}
            onDeleted={onDeleted}
            onClose={() => setEditing(null)}
          />
          )}
        </AnimatePresence>,
        document.body,
      )}
    </div>
  );
}

function DeckToolbar({ api, deck, decks, onDecksChanged }: ManageViewProps) {
  const [mode, setMode] = useState<"view" | "rename" | "new" | "delete">("view");
  const [name, setName] = useState("");
  const [error, setError] = useState<string | null>(null);

  const run = async (action: () => Promise<string | undefined>) => {
    setError(null);
    try {
      const selectID = await action();
      setMode("view");
      onDecksChanged(selectID);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Something went wrong");
    }
  };

  return (
    <div className="deck-toolbar">
      {mode === "view" && (
        <>
          <h2>{deck.name}</h2>
          <span className="deck-toolbar-count">
            {deck.card_count} {deck.card_count === 1 ? "card" : "cards"}
          </span>
          <span className="spacer" />
          <button
            type="button"
            className="btn btn-quiet"
            onClick={() => {
              setName(deck.name);
              setMode("rename");
            }}
          >
            Rename
          </button>
          <button
            type="button"
            className="btn btn-quiet"
            onClick={() => {
              setName("");
              setMode("new");
            }}
          >
            New deck
          </button>
          {decks.length > 1 && (
            <button type="button" className="btn btn-quiet btn-danger-text" onClick={() => setMode("delete")}>
              Delete deck
            </button>
          )}
        </>
      )}
      {(mode === "rename" || mode === "new") && (
        <form
          className="inline-form"
          onSubmit={(e) => {
            e.preventDefault();
            if (!name.trim()) return;
            run(async () =>
              mode === "rename" ? (await api.renameDeck(deck.id, name.trim()), undefined) : (await api.createDeck(name.trim())).id,
            );
          }}
        >
          <input value={name} onChange={(e) => setName(e.target.value)} placeholder="Deck name" autoFocus maxLength={60} />
          <button type="submit" className="btn btn-primary">
            {mode === "rename" ? "Rename" : "Create"}
          </button>
          <button type="button" className="btn btn-quiet" onClick={() => setMode("view")}>
            Cancel
          </button>
        </form>
      )}
      {mode === "delete" && (
        <div className="inline-form">
          <span>
            Delete <strong>{deck.name}</strong> and its {deck.card_count} cards?
          </span>
          <button
            type="button"
            className="btn btn-danger"
            onClick={() => run(async () => (await api.deleteDeck(deck.id), decks.find((d) => d.id !== deck.id)?.id))}
          >
            Delete
          </button>
          <button type="button" className="btn btn-quiet" onClick={() => setMode("view")}>
            Keep
          </button>
        </div>
      )}
      {error && <p className="error-text">{error}</p>}
    </div>
  );
}
