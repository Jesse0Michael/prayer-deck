# AGENTS.md

Guidance for agents and contributors working in this repository. The full
original product brief is in [docs/product-brief.md](docs/product-brief.md).
Decisions made since then are logged in [docs/decisions.md](docs/decisions.md).

## What Prayer Deck is

A personal Christian prayer app built around illustrated cards of friends,
relatives, couples, and families. You shuffle a deck, draw three cards, reveal
the people on them, and spend time praying for them. It is web-first, with an
iOS release in mind.

Keep the product centered on two activities:

- **Draw cards**: shuffle, deal, reveal, and spend time with people in prayer.
- **Manage cards**: add, view, edit, and remove cards and their artwork.

The app should feel personal, warm, and tactile. **Do not** turn it into a
productivity dashboard or a measure of religious performance. Streaks, scores,
competitive features, social feeds, journals, prayer requests, devotionals, and
extensive tracking are out of scope unless the owner asks for them. Do not
expand the feature list just because prayer apps commonly have these features.
Do not redirect the project toward printing physical cards.

### Visual quality is a first-version feature

Card interactions (shuffle, deal, flip, stack, lift, depth) should be fluid and
satisfying, with touch support, a desktop pointer, and a phone-friendly layout.
Use perspective and motion thoughtfully. The interface must work well with still
artwork alone. Loading media must never interrupt a reveal or leave a face
unexpectedly blank, so preload faces before dealing. Validate performance on
representative devices rather than promising every device is lag-free. Honor
`prefers-reduced-motion`.

### Artwork direction

The reference is the original paper deck: simple black pen drawings, sparse
color, handwritten names, and generous blank space. Recognizable features and
personality matter more than realism. The two example illustrations in
`docs/art/` are references, not the chosen style. See
[docs/art-direction.md](docs/art-direction.md). Artwork is generated outside the
app and uploaded for now. In-app generation, LoRA-consistent styles, variants,
and animated faces are future possibilities. Leave room for them, but do not
build their infrastructure ahead of time.

## Decisions (owner's)

- **Go** backend, **PostgreSQL** data, **ent** for entities. Don't replace ent
  because something newer exists; propose a change only with a concrete benefit.
- **Google sign-in** first, with no custom password system. Sign in with Apple
  comes before an App Store release. Adding a provider must attach to the same
  account and collection (see `Identity`).
- **Wails v3** for a shared web/iOS client. Web is the priority; desktop is low
  priority.
- **S3-compatible** media storage, with **MinIO** locally. The production
  provider is undecided.
- **Local testing through Docker.**
- Reuse **`github.com/jesse0michael/pkg`** wherever it doesn't compete with ent or
  Wails. When something here would be useful across projects, propose moving it
  into `pkg` (see [docs/pkg-follow-ups.md](docs/pkg-follow-ups.md)) rather than
  growing local copies.
- Independent of other game work. Do not inherit Unity, Godot, scenes, or gRPC.

Open choices are tracked in [docs/decisions.md](docs/decisions.md). Resolve them
with the owner when they become relevant, and explain tradeoffs in plain
language. Keep the owner's decisions separate from your recommendations.

## Repository layout

```text
api/openapi.yaml        REST contract, the source of truth for the TS client types
cmd/server              API entrypoint (boot.App runner); builds to bin/prayer-deck-api
internal/api            HTTP handlers, routing, CORS, request logging
internal/store          Store interface + ent implementation (all queries scoped to the user)
internal/media          S3/MinIO storage + image renditions (original, display, thumbnail)
internal/ent/schema     ent schemas; generated code in internal/ent is committed
app/                    Wails v3 client (separate Go module)
  main.go               Wails app + ConfigService (runtime API URL)
  frontend/             React + TypeScript + Vite + motion
    src/api/schema.d.ts generated from api/openapi.yaml; never edit by hand
    bindings/           generated Wails bindings; never edit by hand
docs/                   product brief, decisions, art direction, pkg follow-ups
```

### Domain model

`User` ← `Identity` (provider + subject, unique) · `User` → `Deck` → `Card` → `Media`
(one picture; `kind` is `image` for now) → `Asset` (one row per rendition) ·
`Deck` → `Draw` (only the latest is kept, with `card_ids` in dealt order and
`faces` naming the media dealt for each card).

A card's media is listed newest first, and the first item is its primary face.
Drawing picks one of a card's media at random as its face for that draw.

A deck's **pass** is defined by `Deck.shuffled_at`: a card is still in the pass
when `Card.last_drawn_at` is null or earlier. Drawing deals up to the requested
count from the pass and never reshuffles on its own, so a hand can come up short
and a finished pass returns 409. Cards drawn in the pass, other than the current
hand, form the **discard** returned with each draw. Drawing with `shuffle: true`
(or `POST /v1/decks/{id}/shuffle`) starts a fresh pass (`shuffled_at = now`).

## Commands

```bash
make up          # full stack in Docker: web :3000, API :8080, MinIO :9000/:9001, Postgres :5434
make down        # stop and delete local volumes
make run         # API on the host against Docker Postgres + MinIO
make dev-web     # frontend hot reload at http://127.0.0.1:9245
make test        # go tests (sqlite for the store, gomock for handlers)
make lint        # golangci-lint, govulncheck, frontend typecheck
make generate    # ent, mocks, TS API types, Wails bindings
make tools       # install the wails3 CLI into bin/
```

When you change an endpoint, update the handler **and** `api/openapi.yaml`, then
run `make generate` so the frontend types follow. When you change a schema, run
`make generate`. The API applies migrations automatically at startup
(`Schema.Create`), which is acceptable only until real data exists; see decisions.

## Go conventions

- Wrap errors with context: `fmt.Errorf("failed to do x: %w", err)`.
- Log with `slog` and context, and always use the `"err"` attribute:
  `slog.ErrorContext(ctx, "failed to x", "err", err)`.
- Comments explain *why*, never *what*, and never describe history ("was",
  "changed", "now"). Default to one line. Document exported identifiers and
  interface methods.
- Favor readability over brevity. Add a helper only when it reduces complexity,
  not merely to remove duplication.
- After editing Go files, run `goimports -w <files>` and
  `go tool golangci-lint run --fix ./...`.
- Build binaries into the root `bin/` directory.

## Testing conventions

- Table-driven tests. Add cases to the existing table for a function rather than
  writing a new test for one scenario.
- Use `t.Context()`. Mock with `gomock` through a `mockSetup` field, set in every
  case.
- Identifiers look like `test-{type}` or `test-{type}-{index}`.
- Put fixtures in `testdata/` and load them with `test.LoadFile` /
  `test.LoadJSONFile` from `pkg/test`.
- Compare full values (`require.Equal`, `cmp.Diff`), and use exact response
  bodies in handler tests.
- Keep tests deterministic: `synctest.Test` for time, `uuid.SetRand` with a
  seeded source for IDs.

## Frontend conventions

- All card motion runs through `motion` springs on transforms only, in a single
  stage coordinate space (see `DrawView.computeLayout`). Avoid layout-thrashing
  properties in animations.
- Preload and decode card faces (`lib/media.ts`) before a reveal can show them.
- Overlays render in a portal on `document.body`. Set `backdrop-filter` inline,
  because the CSS minifier drops the unprefixed property.
- The frontend reads its API URL from the Wails `ConfigService` at runtime and
  falls back to `VITE_API_URL` outside Wails.
