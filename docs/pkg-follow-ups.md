# pkg follow-ups

Things found while building Prayer Deck on `github.com/jesse0michael/pkg` that
would be better fixed or shared there. Nothing has been changed in `pkg`.

## Fixes

- **`boot`: config errors exit silently.** When `config.New` fails,
  `NewApp` cancels the context and `Run` returns the error without logging it.
  A container just restarts with no output. Log the cause before returning.
- **`config` (v0.11.0): named sub-config fields break CLI parsing.** A field like
  `Postgres config.PostgresConfig` makes go-arg fail with "fields are not
  supported". The workaround is tagging every nested config with `arg:"-"`.
  Consider skipping struct fields in `parseArgs` or supporting them, and update
  the evoke-style `Config` examples.
- **`http/middleware.Auth`** answers 403 for a missing or expired token where 401
  is the conventional status, and it sets `Content-Type` after `WriteHeader`
  (so the header is never sent). `RejectReadOnly` and `RevokedToken` have the
  same header-ordering bug.
- **`http/errors.WriteError`** doesn't set `Content-Type: application/json`.
- **`config.NewPostgresClient`** logs `invalid connection config: missing required
  peer IP or hostname` at startup through the Splunk instrumentation, which is
  noisy and misleading.

## Enhancements

- **`config.PostgresConfig`: accept a single connection URL.** Managed hosts such
  as Neon hand out one `postgresql://user:password@host/db?sslmode=require`
  string, but the config only reads the discrete `POSTGRES_HOST`, `POSTGRES_USER`,
  `POSTGRES_PASSWORD`, `POSTGRES_DB`, and `POSTGRES_SSLMODE` fields (v0.11.0), so
  the URL has to be taken apart by hand. Add an optional `POSTGRES_URL` (or
  `DATABASE_URL`) field that, when set, supplies every connection parameter and
  takes precedence over the discrete fields; `ConnectionString()` can return it
  directly. The pool settings (`POSTGRES_MAX_CONNS` and friends) stay separate.
  Prayer Deck would then deploy with one secret instead of five variables and
  could drop the split in `deploy/cloudrun.tf`. Use the direct (non-pooled) Neon
  URL; the pooled one runs through PgBouncer in transaction mode.

## Candidates to move into pkg

- **CORS middleware** (`internal/api/middleware.go`): an allow-list of origins
  with preflight handling. Every browser-facing service needs one.
- **Request logging middleware**: method, path, status, and duration through
  `slog`, matching the gRPC log interceptor.
- **`oidc.DevVerifier`** (`internal/api/accounts.go`): an `oidc.Verifier` that
  accepts an email address for local development, so any app using
  `HandleOIDCLogin` can run without provider credentials.
- **S3 storage** (`internal/media/storage.go`): `Put` / `Delete` / signed `URL`
  with a separate public endpoint for signing behind Docker or a proxy, plus a
  `HealthChecker`. This could sit beside `config.PostgresConfig` as an
  `S3Config`.
