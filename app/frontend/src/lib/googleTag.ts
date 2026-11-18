export type DeckMode = "demo" | "personal";
type Page = "landing" | "sign-in" | "draw" | "cards";
type CardEvent = "card_created" | "card_updated" | "card_deleted";

declare global {
  interface Window {
    dataLayer?: IArguments[];
  }
}

const titles: Record<Page, string> = {
  landing: "Prayer Deck",
  "sign-in": "Sign in | Prayer Deck",
  draw: "Draw | Prayer Deck",
  cards: "Cards | Prayer Deck",
};

interface AnalyticsSettings {
  measurementId?: string;
  hostname?: string;
  production: boolean;
  debugRequested: boolean;
}

export function createAnalytics(settings: AnalyticsSettings, browser: Window) {
  const { measurementId, hostname, production, debugRequested } = settings;
  const debug = !production && debugRequested;
  const enabled = /^G-[A-Z0-9]+$/.test(measurementId ?? "") && (
    debug || (production && browser.location.protocol === "https:" && browser.location.hostname === hostname)
  );
  let currentPage: Page | undefined;

  function gtag(..._args: unknown[]) {
    (browser.dataLayer ??= []).push(arguments);
  }

  /** Only the four named views may supply page metadata, including for automatic GA events. */
  function trackPage(page: Page) {
    if (!enabled || page === currentPage) return;
    const first = currentPage === undefined;
    currentPage = page;
    try {
      if (first) gtag("js", new Date());
      gtag("config", measurementId, {
        update: !first,
        send_page_view: false,
        page_location: `${browser.location.origin}/${page === "landing" ? "" : page}`,
        page_title: titles[page],
        page_referrer: "",
        allow_google_signals: false,
        allow_ad_personalization_signals: false,
      });
      gtag("event", "page_view", { send_to: measurementId });
      if (first && !debug) {
        const script = browser.document.createElement("script");
        script.async = true;
        script.referrerPolicy = "no-referrer";
        script.src = `https://www.googletagmanager.com/gtag/js?id=${measurementId}`;
        browser.document.head.appendChild(script);
      }
    } catch {
      // A blocker can replace the data layer or prevent the tag from loading.
    }
  }

  function event(name: string, parameters: Record<string, string | number>) {
    if (!enabled || currentPage === undefined) return;
    try {
      gtag("event", name, { ...parameters, send_to: measurementId });
    } catch {
      // Analytics must never turn a successful action into a failed one.
    }
  }

  function trackLogin() {
    event("login", { method: "google" });
  }

  function trackHandDealt(mode: DeckMode, shuffle: boolean, size: number) {
    event("hand_dealt", { deck_mode: mode, draw_type: shuffle ? "shuffle" : "next", hand_size: size });
  }

  function trackCard(action: CardEvent, mode: DeckMode) {
    event(action, { deck_mode: mode });
  }

  return { trackPage, trackLogin, trackHandDealt, trackCard };
}
