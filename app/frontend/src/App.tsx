import { AnimatePresence, MotionConfig, motion } from "motion/react";
import { useCallback, useEffect, useState } from "react";
import { ApiClient } from "./api/client";
import type { Deck, User } from "./api/types";
import { loadAPIURL } from "./lib/config";
import { DrawView } from "./views/DrawView";
import { ManageView } from "./views/ManageView";
import { SignIn } from "./views/SignIn";

type View = "draw" | "manage";

const SELECTED_DECK_KEY = "prayer-deck.deck";

export function App() {
  const [api, setApi] = useState<ApiClient | null>(null);
  const [signedIn, setSignedIn] = useState(false);

  useEffect(() => {
    loadAPIURL().then((url) => {
      const client = new ApiClient(url);
      setSignedIn(client.signedIn);
      client.onAuthChange(setSignedIn);
      setApi(client);
    });
  }, []);

  return (
    <MotionConfig reducedMotion="user">
      <div className="table-surface" />
      {!api ? null : signedIn ? <Home api={api} /> : <SignIn api={api} />}
    </MotionConfig>
  );
}

function Home({ api }: { api: ApiClient }) {
  const [user, setUser] = useState<User | null>(null);
  const [decks, setDecks] = useState<Deck[]>([]);
  const [deckID, setDeckID] = useState<string | null>(() => localStorage.getItem(SELECTED_DECK_KEY));
  const [view, setView] = useState<View>("draw");
  const [error, setError] = useState<string | null>(null);

  const refreshDecks = useCallback(
    (selectDeckID?: string) => {
      api
        .decks()
        .then((list) => {
          setDecks(list);
          setDeckID((current) => {
            const wanted = selectDeckID ?? current;
            return list.some((d) => d.id === wanted) ? wanted : (list[0]?.id ?? null);
          });
        })
        .catch((err: Error) => setError(err.message));
    },
    [api],
  );

  useEffect(() => {
    api.me().then(setUser).catch(() => undefined);
    refreshDecks();
  }, [api, refreshDecks]);

  useEffect(() => {
    if (deckID) localStorage.setItem(SELECTED_DECK_KEY, deckID);
  }, [deckID]);

  const deck = decks.find((d) => d.id === deckID);

  return (
    <div className="app">
      <header className="app-header">
        <span className="hand-title brand">Prayer Deck</span>
        <nav className="tabs" aria-label="Sections">
          {(["draw", "manage"] as const).map((v) => (
            <button key={v} type="button" className={`tab${view === v ? " active" : ""}`} onClick={() => setView(v)}>
              {view === v && <motion.span layoutId="tab-ink" className="tab-ink" transition={{ type: "spring", stiffness: 380, damping: 30 }} />}
              <span className="tab-label">{v === "draw" ? "Draw" : "Cards"}</span>
            </button>
          ))}
        </nav>
        <span className="spacer" />
        {decks.length > 1 && (
          <select className="deck-select" value={deckID ?? ""} onChange={(e) => setDeckID(e.target.value)} aria-label="Deck">
            {decks.map((d) => (
              <option key={d.id} value={d.id}>
                {d.name}
              </option>
            ))}
          </select>
        )}
        <button type="button" className="btn btn-quiet account" onClick={() => api.signOut()} title={user?.email}>
          Sign out
        </button>
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
                <ManageView api={api} deck={deck} decks={decks} onDecksChanged={refreshDecks} />
              )}
            </motion.div>
          )}
        </AnimatePresence>
      </main>
    </div>
  );
}
