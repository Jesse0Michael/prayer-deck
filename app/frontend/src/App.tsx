import { MotionConfig } from "motion/react";
import { useEffect, useState } from "react";
import { CachedDeckApi } from "./api/cachedApi";
import { ApiClient } from "./api/client";
import type { User } from "./api/types";
import { loadAPIURL } from "./lib/config";
import { DeckApp } from "./views/DeckApp";
import { Landing } from "./views/Landing";
import { SignIn } from "./views/SignIn";

/** The current location hash, e.g. "#/about"; empty when at the root. */
function useHash(): string {
  const [hash, setHash] = useState(window.location.hash);
  useEffect(() => {
    const onChange = () => {
      setHash(window.location.hash);
      window.scrollTo(0, 0);
    };
    window.addEventListener("hashchange", onChange);
    return () => window.removeEventListener("hashchange", onChange);
  }, []);
  return hash;
}

export function App() {
  const [api, setApi] = useState<ApiClient | null>(null);
  const [deckApi, setDeckApi] = useState<CachedDeckApi | null>(null);
  const [signedIn, setSignedIn] = useState(false);
  const hash = useHash();

  useEffect(() => {
    loadAPIURL().then((url) => {
      const client = new ApiClient(url);
      setSignedIn(client.signedIn);
      client.onAuthChange(setSignedIn);
      setDeckApi(new CachedDeckApi(client));
      setApi(client);
    });
  }, []);

  return (
    <MotionConfig reducedMotion="user">
      <div className="table-surface" />
      {!api || !deckApi ? null : hash === "#/about" ? (
        <Landing signedIn={signedIn} />
      ) : signedIn ? (
        <Home api={api} deckApi={deckApi} />
      ) : hash === "#/signin" ? (
        <SignIn api={api} />
      ) : (
        <Landing signedIn={false} />
      )}
    </MotionConfig>
  );
}

function Home({ api, deckApi }: { api: ApiClient; deckApi: CachedDeckApi }) {
  const [user, setUser] = useState<User | null>(null);

  useEffect(() => {
    deckApi.me().then(setUser).catch(() => undefined);
  }, [deckApi]);

  return (
    <DeckApp
      api={deckApi}
      actions={
        <button type="button" className="btn btn-quiet account" onClick={() => api.signOut()} title={user?.email}>
          Sign out
        </button>
      }
    />
  );
}
