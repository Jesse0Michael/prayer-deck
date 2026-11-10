import { motion } from "motion/react";
import { useEffect, useRef, useState, type FormEvent } from "react";
import type { ApiClient } from "../api/client";
import type { LoginProvider } from "../api/types";
import { CardBack } from "../components/CardBack";

interface GoogleIdentity {
  accounts: {
    id: {
      initialize(options: { client_id: string; callback: (response: { credential: string }) => void }): void;
      renderButton(el: HTMLElement, options: Record<string, string | number>): void;
    };
  };
}

declare global {
  interface Window {
    google?: GoogleIdentity;
  }
}

function loadGoogleIdentity(): Promise<GoogleIdentity> {
  if (window.google) return Promise.resolve(window.google);
  return new Promise((resolve, reject) => {
    const script = document.createElement("script");
    script.src = "https://accounts.google.com/gsi/client";
    script.async = true;
    script.onload = () => (window.google ? resolve(window.google) : reject(new Error("Google sign-in did not load")));
    script.onerror = () => reject(new Error("Google sign-in could not be reached"));
    document.head.appendChild(script);
  });
}

export function SignIn({ api }: { api: ApiClient }) {
  const [providers, setProviders] = useState<LoginProvider[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [email, setEmail] = useState("");
  const googleButton = useRef<HTMLDivElement>(null);

  useEffect(() => {
    api
      .loginProviders()
      .then(setProviders)
      .catch(() => setError("Prayer Deck can't reach its server right now."));
  }, [api]);

  const google = providers?.find((p) => p.name === "google");
  const dev = providers?.find((p) => p.name === "dev");

  useEffect(() => {
    if (!google?.client_id || !googleButton.current) return;
    const el = googleButton.current;
    loadGoogleIdentity()
      .then((gis) => {
        gis.accounts.id.initialize({
          client_id: google.client_id!,
          callback: ({ credential }) => {
            api.signIn("google", credential).catch((err: Error) => setError(err.message));
          },
        });
        gis.accounts.id.renderButton(el, { theme: "outline", size: "large", shape: "pill", text: "continue_with", width: 280 });
      })
      .catch((err: Error) => setError(err.message));
  }, [api, google?.client_id]);

  const devSignIn = (e: FormEvent) => {
    e.preventDefault();
    api.signIn("dev", email).catch((err: Error) => setError(err.message));
  };

  return (
    <div className="signin">
      <div className="signin-fan" aria-hidden="true">
        {[-14, 0, 14].map((rotate, i) => (
          <motion.div
            key={rotate}
            className="signin-fan-card"
            initial={{ y: 60, rotate: 0, opacity: 0 }}
            animate={{ y: Math.abs(rotate) / 2, rotate, opacity: 1 }}
            transition={{ type: "spring", stiffness: 90, damping: 14, delay: 0.15 + i * 0.12 }}
          >
            <CardBack />
          </motion.div>
        ))}
      </div>
      <motion.div className="signin-panel" initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.4 }}>
        <h1 className="hand-title">Prayer Deck</h1>
        <p className="signin-tagline">Spend a few quiet minutes with the people you love.</p>
        {google && <div ref={googleButton} className="google-button" />}
        {dev && (
          <form className="dev-signin" onSubmit={devSignIn}>
            <span className="dev-label">Local development sign-in</span>
            <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="you@example.com" required />
            <button type="submit" className="btn btn-primary">
              Continue
            </button>
          </form>
        )}
        {providers && providers.length === 0 && <p className="error-text">No sign-in methods are configured on the server.</p>}
        {error && <p className="error-text">{error}</p>}
        <a className="signin-about" href="#/about">
          What is Prayer Deck?
        </a>
      </motion.div>
    </div>
  );
}
