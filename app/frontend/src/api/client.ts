import type { Card, CardInput, Deck, Draw, LoginProvider, Tokens, User } from "./types";

const TOKENS_KEY = "prayer-deck.tokens";

export class ApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message);
  }
}

type Listener = (signedIn: boolean) => void;

/**
 * ApiClient talks to the Prayer Deck API. It keeps the token pair in local
 * storage and transparently refreshes an expired access token once per request.
 */
export class ApiClient {
  private tokens: Tokens | null;
  private refreshing: Promise<boolean> | null = null;
  private listeners = new Set<Listener>();

  constructor(readonly baseURL: string) {
    const stored = localStorage.getItem(TOKENS_KEY);
    this.tokens = stored ? (JSON.parse(stored) as Tokens) : null;
  }

  get signedIn(): boolean {
    return this.tokens !== null;
  }

  onAuthChange(listener: Listener): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  signOut(): void {
    this.setTokens(null);
  }

  loginProviders(): Promise<LoginProvider[]> {
    return this.request<{ providers: LoginProvider[] }>("GET", "/v1/tokens", { auth: false }).then((r) => r.providers);
  }

  async signIn(provider: string, idToken: string): Promise<void> {
    const tokens = await this.request<Tokens>("POST", `/v1/tokens/${provider}`, {
      auth: false,
      body: { id_token: idToken },
    });
    this.setTokens(tokens);
  }

  me(): Promise<User> {
    return this.request("GET", "/v1/me");
  }

  decks(): Promise<Deck[]> {
    return this.request<{ decks: Deck[] }>("GET", "/v1/decks").then((r) => r.decks);
  }

  createDeck(name: string): Promise<Deck> {
    return this.request("POST", "/v1/decks", { body: { name } });
  }

  renameDeck(deckID: string, name: string): Promise<Deck> {
    return this.request("PATCH", `/v1/decks/${deckID}`, { body: { name } });
  }

  deleteDeck(deckID: string): Promise<void> {
    return this.request("DELETE", `/v1/decks/${deckID}`);
  }

  shuffle(deckID: string): Promise<Deck> {
    return this.request("POST", `/v1/decks/${deckID}/shuffle`);
  }

  cards(deckID: string): Promise<Card[]> {
    return this.request<{ cards: Card[] }>("GET", `/v1/decks/${deckID}/cards`).then((r) => r.cards);
  }

  createCard(deckID: string, input: CardInput & { name: string }): Promise<Card> {
    return this.request("POST", `/v1/decks/${deckID}/cards`, { body: input });
  }

  updateCard(cardID: string, input: CardInput): Promise<Card> {
    return this.request("PATCH", `/v1/cards/${cardID}`, { body: input });
  }

  deleteCard(cardID: string): Promise<void> {
    return this.request("DELETE", `/v1/cards/${cardID}`);
  }

  /** Uploads an image as the card's newest media, which becomes its primary face. */
  addMedia(cardID: string, file: Blob): Promise<Card> {
    const form = new FormData();
    form.append("image", file);
    return this.request("POST", `/v1/cards/${cardID}/media`, { form });
  }

  deleteMedia(cardID: string, mediaID: string): Promise<void> {
    return this.request("DELETE", `/v1/cards/${cardID}/media/${mediaID}`);
  }

  /** Deals the next cards in the pass; with shuffle, every card returns to the pass first. */
  draw(deckID: string, count: number, shuffle: boolean): Promise<Draw> {
    return this.request("POST", `/v1/decks/${deckID}/draws`, { body: { count, shuffle } });
  }

  /** The deck's most recent draw, or null when nothing has been drawn yet. */
  async currentDraw(deckID: string): Promise<Draw | null> {
    try {
      return await this.request<Draw>("GET", `/v1/decks/${deckID}/draws/current`);
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) return null;
      throw err;
    }
  }

  private setTokens(tokens: Tokens | null): void {
    const changed = (tokens === null) !== (this.tokens === null);
    this.tokens = tokens;
    if (tokens) localStorage.setItem(TOKENS_KEY, JSON.stringify(tokens));
    else localStorage.removeItem(TOKENS_KEY);
    if (changed) this.listeners.forEach((l) => l(tokens !== null));
  }

  private async request<T>(
    method: string,
    path: string,
    opts: { auth?: boolean; body?: unknown; form?: FormData; retried?: boolean } = {},
  ): Promise<T> {
    const auth = opts.auth ?? true;
    const headers: Record<string, string> = {};
    if (auth && this.tokens) headers.Authorization = `Bearer ${this.tokens.access_token}`;
    let body: BodyInit | undefined;
    if (opts.form) {
      body = opts.form;
    } else if (opts.body !== undefined) {
      headers["Content-Type"] = "application/json";
      body = JSON.stringify(opts.body);
    }

    const res = await fetch(this.baseURL + path, { method, headers, body });

    // The API answers 403 for any unusable access token, including an expired one.
    if (auth && (res.status === 401 || res.status === 403) && !opts.retried) {
      if (await this.refresh()) return this.request<T>(method, path, { ...opts, retried: true });
      this.setTokens(null);
    }
    if (!res.ok) {
      throw new ApiError(res.status, await errorMessage(res));
    }
    if (res.status === 204) return undefined as T;
    return (await res.json()) as T;
  }

  /** Exchanges the refresh token for a new pair; concurrent callers share one exchange. */
  private refresh(): Promise<boolean> {
    if (!this.tokens) return Promise.resolve(false);
    if (!this.refreshing) {
      const refreshToken = this.tokens.refresh_token;
      this.refreshing = fetch(this.baseURL + "/v1/tokens/refresh", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ refresh_token: refreshToken }),
      })
        .then(async (res) => {
          if (!res.ok) return false;
          this.setTokens((await res.json()) as Tokens);
          return true;
        })
        .catch(() => false)
        .finally(() => {
          this.refreshing = null;
        });
    }
    return this.refreshing;
  }
}

async function errorMessage(res: Response): Promise<string> {
  try {
    const data = (await res.json()) as { errors?: { message: string; details?: string }[] };
    const first = data.errors?.[0];
    if (first) return first.details ? `${first.message}: ${first.details}` : first.message;
  } catch {
    // Non-JSON error bodies fall through to the status text.
  }
  return res.statusText || `Request failed (${res.status})`;
}
