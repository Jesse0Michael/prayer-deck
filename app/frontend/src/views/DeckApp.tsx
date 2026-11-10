import { AnimatePresence, motion } from "motion/react";
import { useCallback, useEffect, useState, type ReactNode } from "react";
import type { DeckApi } from "../api/deckApi";
import type { Deck } from "../api/types";
import { DrawView } from "./DrawView";
import { ManageView } from "./ManageView";

type View = "draw" | "manage";

interface DeckAppProps {
  api: DeckApi;
  /** Controls at the right end of the header. */
  actions: ReactNode;
  /** Fills its container rather than the viewport. */
  embedded?: boolean;
}

/** The signed-in app: a header with Draw and Cards tabs over the account's deck. */
export function DeckApp({ api, actions, embedded = false }: DeckAppProps) {
  const [decks, setDecks] = useState<Deck[]>([]);
  const [view, setView] = useState<View>("draw");
  const [error, setError] = useState<string | null>(null);

  const refreshDecks = useCallback(() => {
    api
      .decks()
      .then(setDecks)
      .catch((err: Error) => setError(err.message));
  }, [api]);

  useEffect(() => {
    refreshDecks();
  }, [refreshDecks]);

  // The UI presents a single deck; the first one holds every card.
  const deck = decks[0];

  return (
    <div className={`app${embedded ? " app-embedded" : ""}`}>
      <header className="app-header">
        <a className="hand-title brand" href="#/about" title="About Prayer Deck">
          Prayer Deck
        </a>
        <nav className="tabs" aria-label="Sections">
          {(["draw", "manage"] as const).map((v) => (
            <button key={v} type="button" className={`tab${view === v ? " active" : ""}`} onClick={() => setView(v)}>
              {view === v && <motion.span layoutId="tab-ink" className="tab-ink" transition={{ type: "spring", stiffness: 380, damping: 30 }} />}
              <span className="tab-label">{v === "draw" ? "Draw" : "Cards"}</span>
            </button>
          ))}
        </nav>
        <span className="spacer" />
        {actions}
      </header>

      {error && <p className="error-text">{error}</p>}

      <main className="app-main">
        <AnimatePresence mode="wait">
          {deck && (
            <motion.div
              key={`${view}-${deck.id}`}
              className="view"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              transition={{ duration: 0.25 }}
            >
              {view === "draw" ? (
                <DrawView api={api} deck={deck} onDeckChanged={refreshDecks} onManage={() => setView("manage")} />
              ) : (
                <ManageView api={api} deck={deck} onDecksChanged={refreshDecks} />
              )}
            </motion.div>
          )}
        </AnimatePresence>
      </main>
    </div>
  );
}
