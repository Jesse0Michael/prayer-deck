import { AnimatePresence, MotionConfig, motion } from "motion/react";
import { useCallback, useEffect, useState } from "react";
import { ApiClient } from "./api/client";
import type { Deck, User } from "./api/types";
import { loadAPIURL } from "./lib/config";
import { DrawView } from "./views/DrawView";
import { ManageView } from "./views/ManageView";
import { SignIn } from "./views/SignIn";

type View = "draw" | "manage";

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
  const [view, setView] = useState<View>("draw");
  const [error, setError] = useState<string | null>(null);

  const refreshDecks = useCallback(() => {
    api
      .decks()
      .then(setDecks)
      .catch((err: Error) => setError(err.message));
  }, [api]);

  useEffect(() => {
    api.me().then(setUser).catch(() => undefined);
    refreshDecks();
  }, [api, refreshDecks]);

  // The UI presents a single deck; the first one holds every card.
  const deck = decks[0];

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
                <ManageView api={api} deck={deck} onDecksChanged={refreshDecks} />
              )}
            </motion.div>
          )}
        </AnimatePresence>
      </main>
    </div>
  );
}
