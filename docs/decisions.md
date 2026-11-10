# Decisions

A running log. **Owner** marks a decision the owner made. **Proposed** marks a
recommendation made during the MVP build that the owner hasn't confirmed yet.
Revisit proposals when they start to matter.

## Settled by the owner

- Go backend, PostgreSQL, ent.
- Google sign-in first; Sign in with Apple before an App Store release.
- Wails v3 for the client, web first.
- S3-compatible artwork storage, MinIO locally, everything runnable in Docker.
- Build on `github.com/jesse0michael/pkg`.

## Proposed in the MVP

### Wails v3 delivery path

Checked against v3.1.0 (the stable release as of November 2026):

- **Web works today.** The client is built with `-tags server`, so Wails serves
  the frontend over HTTP to ordinary browsers, and frontend calls to Go
  services travel over HTTP. This repo uses that path in Docker. The
  `ConfigService` binding is called from the browser to confirm the round trip.
  Upstream still labels server mode experimental.
- **iOS is experimental upstream.** It renders the same frontend in a WKWebView
  and requires macOS with full Xcode. The template's `build/ios` tasks are kept
  (`wails3 task ios:run`) but haven't been exercised yet. **Next step:** run the
  iOS simulator build before investing further in platform-specific polish.
- **Known iOS risk:** Google blocks OAuth inside embedded web views, so the
  Google Identity Services button that works on the web won't work in the iOS
  app. iOS will need a native sign-in hand-off (system browser with
  ASWebAuthenticationSession, or Sign in with Apple) that ends with the same
  `POST /v1/tokens/{provider}` exchange. The API is already provider-agnostic.
- Wails services run inside the client process. Shared Go code in the client
  doesn't synchronize data; everything goes through the hosted API.

### Multiple decks: supported, kept light

The schema and API support many decks per account. For now the UI shows no deck
controls at all: every account starts with "My Deck", and the app draws from and
manages the account's first deck.

### Shuffle and repeat behavior

Each deck is drawn in **passes**: a card isn't dealt again until every card in
the deck has been dealt once, or the deck is shuffled. The table offers two
actions, being tried out side by side:

- **Shuffle** gathers the hand and the discard pile back into the deck, shuffles,
  and deals three from a fresh pass.
- **Draw** lays the hand face up on a discard pile beside the deck and deals the
  next (up to) three from the same pass. When the pass is finished, only Shuffle
  is available.

### Draw persistence

Only the latest draw per deck is stored, enough to resume prayer time after
closing the app. Which cards have been turned over is remembered per device in
local storage. No draw history is kept, by design (no tracking).

### Multiple pictures per card (Owner)

A card holds any number of `Media` items, each one picture with its own
renditions. Media is ordered newest first, and the newest is the card's primary
face, shown in lists and the editor. Uploading adds a new primary, and nothing
is replaced. `Media.kind` is `image` today; video can join as another kind.

When a card is dealt, the server picks one of its media at random and records
it in the draw's `faces`, so resuming a draw shows the same pictures. The
discard pile shows each card's primary face, because only the latest draw is
kept.

The editor shows the primary image above a gallery of the rest (two rows of
three per page). Gallery images can be removed, and the primary can't be
removed from the UI until a newer image takes its place. Picked images and
removals are staged locally and applied on Save, so Cancel leaves the card
untouched.

### Image renditions

Each upload is stored three ways: the untouched **original**, a JPEG **display**
image (at most 1600px on its long edge, used for the card face), and a
**thumbnail** (at most 480px, used in lists). A 2.3 MB example PNG becomes a
roughly 240 KB face. Transparent artwork is laid on the card-stock color.
`Asset.rendition` is an enum, so alternate sizes can join later as new
renditions.

Artwork is served through time-limited signed URLs (six hours by default). Signed
URLs change on every response, which limits browser caching. Before launch,
consider a CDN or deterministic signing.

### Sign-in for local development

`DEV_LOGIN=true` adds a `dev` provider that accepts an email address as its
token. It lets the app run end to end without Google credentials. It must never
be enabled in a deployed environment.

### Account linking

A new provider identity joins an existing account only when the provider marks
the email as verified. Apple's private relay emails won't match a Google email,
so linking those needs an explicit "link another sign-in" flow while signed in.
That flow isn't built yet.

### Migrations

The API runs ent's automatic migration at startup. Switch to versioned
migrations (Atlas) before production data exists. **To do:** this needs a
decision and setup before the next breaking schema change.

Automatic migration only adds tables and columns. It can't backfill data, so
a change that makes a new column required fails on a database with rows. The
move from `Card → Asset` to `Card → Media → Asset` hit this, and local
databases had to be wiped (`make down && make up`).

Setup outline:

- Enable ent's `sql/versioned-migration` feature and add an `atlas.hcl` that
  points at the ent schema, with a throwaway Postgres as the dev database.
- Generate a baseline migration of the current schema and commit `migrations/`
  with its `atlas.sum`. Mark it applied on any existing database with
  `atlas migrate apply --baseline`.
- Replace `Schema.Create` in `cmd/app/main.go` with `atlas migrate apply`,
  run before the API starts.
- For each schema change, generate a migration with `atlas migrate diff`, then
  hand-edit it to add any data backfill (add the column nullable, backfill,
  then make it required). Run `atlas migrate hash` and `atlas migrate lint`.
- Add `atlas migrate validate` to `make lint`.

## Still open

- Hosting and media storage providers (affordability matters).
- Final visual style, card framing, and name rendering (see [art-direction.md](art-direction.md)).
- Offline behavior.
- Whether "Shuffle" belongs in the UI or the automatic pass is enough.
- Phone layout for the hand: three cards in a row is readable but small on narrow
  phones, so a fanned or overlapping hand may suit them better.
