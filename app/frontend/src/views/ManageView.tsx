import { AnimatePresence, motion } from "motion/react";
import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import type { DeckApi } from "../api/deckApi";
import type { Card, Deck } from "../api/types";
import { CardBack } from "../components/CardBack";
import { CardFace } from "../components/CardFace";
import { useDelayed } from "../lib/useDelayed";
import { CardEditor } from "./CardEditor";

interface ManageViewProps {
  api: DeckApi;
  /** Undefined while the deck is still loading. */
  deck: Deck | undefined;
  onDecksChanged: () => void;
}

type Editing = { card?: Card } | null;

const PLACEHOLDER_COUNT = 5;
const SLOW_LOAD_MS = 2500;

export function ManageView({ api, deck, onDecksChanged }: ManageViewProps) {
  const [cards, setCards] = useState<Card[] | null>(null);
  const [editing, setEditing] = useState<Editing>(null);
  const [error, setError] = useState<string | null>(null);

  const deckID = deck?.id;
  const loading = cards === null && !error;
  const slow = useDelayed(loading, SLOW_LOAD_MS);

  useEffect(() => {
    if (!deckID) return;
    let cancelled = false;
    setCards(null);
    api
      .cards(deckID)
      .then((c) => !cancelled && setCards(c))
      .catch((err: Error) => !cancelled && setError(err.message));
    return () => {
      cancelled = true;
    };
  }, [api, deckID]);

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
      {error && <p className="error-text">{error}</p>}

      <motion.ul className="card-grid" layout>
        <li>
          <button type="button" className="grid-card grid-add" onClick={() => setEditing({})} disabled={!deck}>
            <span className="grid-add-plus">+</span>
            <span>Add a card</span>
          </button>
        </li>
        {loading &&
          Array.from({ length: PLACEHOLDER_COUNT }, (_, i) => (
            <li key={`placeholder-${i}`} aria-hidden="true">
              <motion.div
                className="grid-card grid-placeholder"
                initial={{ opacity: 0.45, y: 0 }}
                animate={{ opacity: [0.45, 0.85, 0.45], y: [0, -8, 0] }}
                transition={{ duration: 1.6, ease: "easeInOut", repeat: Infinity, delay: i * 0.18 }}
              >
                <CardBack />
              </motion.div>
            </li>
          ))}
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
                aria-label={card.name}
                onClick={() => setEditing({ card })}
                whileHover={{ y: -6, rotate: -0.6 }}
                whileTap={{ scale: 0.97 }}
              >
                <CardFace card={card} thumbnail />
              </motion.button>
            </motion.li>
          ))}
        </AnimatePresence>
      </motion.ul>

      <AnimatePresence>
        {slow && (
          <motion.p
            className="loading-note"
            role="status"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
          >
            Gathering your cards…
          </motion.p>
        )}
      </AnimatePresence>

      {createPortal(
        <AnimatePresence>
          {editing && deck && (
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
