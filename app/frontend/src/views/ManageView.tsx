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
  onDecksChanged: () => void;
}

type Editing = { card?: Card } | null;

export function ManageView({ api, deck, onDecksChanged }: ManageViewProps) {
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
