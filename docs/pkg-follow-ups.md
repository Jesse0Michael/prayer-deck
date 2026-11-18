# pkg follow-ups

Suggestions from the Prayer Deck repository sweep, checked against `pkg/config`
v0.12.0, `pkg/boot` v1.3.1, and `pkg/http` v0.8.1 on 2026-11-16. These are
recommendations for `github.com/jesse0michael/pkg`; this work does not change that
repository.

## Already available

- **S3 configuration and clients**: [`config.S3Config`, `NewS3Client`, and
  `NewS3PresignClient`](https://github.com/Jesse0Michael/pkg/blob/config/v0.12.0/config/s3.go)
  cover bucket, region, internal/public endpoints, path-style addressing, and
  compatible checksum settings. `AWSConfig` supplies credentials through the
  AWS SDK. Prayer Deck uses these in `internal/media/storage.go`; a new `S3Config` is no
  longer a follow-up.
- **Postgres connection URLs**: `config.PostgresConfig.URL` accepts
  `POSTGRES_URL`. `deploy/cloudrun.tf` can eventually use one connection secret
  instead of separate connection fields; adding URL support is no longer needed.
- **REST clients**: [`http/client.REST`](https://github.com/Jesse0Michael/pkg/blob/http/v0.8.1/http/client/rest.go)
  already handles default headers, response decoding, body closure, and HTTP
  errors. Consider adopting it in `cmd/seed/main.go` before proposing another
  client helper. Preserve the seed command's timeout and non-retrying client:
  automatically retrying card creation or uploads can create duplicates.

## Fixes to propose

- **Preserve temporary AWS credentials**
  ([`config/aws.go`](https://github.com/Jesse0Michael/pkg/blob/config/v0.12.0/config/aws.go)).
  `NewAWSConfig` installs a static provider with an empty session token whenever
  `AccessKey` and `SecretKey` are populated. Loading those fields from the normal
  AWS environment therefore drops `AWS_SESSION_TOKEN`. Add a session-token
  override or let environment credentials use the SDK's default chain. Test
  access key, secret, and session token together. MinIO/R2 static keys are
  unaffected, but AWS temporary credentials would fail with this configuration.
- **Run cleanup after startup failures**
  ([`boot/app.go`](https://github.com/Jesse0Michael/pkg/blob/boot/v1.3.1/boot/app.go)).
  `App.Run` returns on a cancellation cause before closing runners and telemetry
  providers, and ignores runner `Close` errors on normal shutdown. In
  `cmd/server/main.go`, storage, OIDC discovery, or listener setup can fail after
  the database opens. Ensure initialized resources are closed on both paths,
  retaining the startup error and reporting cleanup errors. Test a runner that
  allocates a resource and then fails.
- **Preserve Postgres URL query options**
  ([`config/postgres.go`](https://github.com/Jesse0Michael/pkg/blob/config/v0.12.0/config/postgres.go)).
  URL resolution copies only `sslmode` from the query; rebuilding the connection
  URL drops driver options such as `connect_timeout`, `application_name`, and
  certificate paths. Preserve supported query parameters when merging a URL
  with discrete defaults, and test escaped values. This is a gap in the new URL
  support, separate from the completed request to add it.

## Helpers worth sharing

- **CORS middleware** (`internal/api/middleware.go`): configurable allowed origins,
  methods, headers, and preflight cache duration in `http/middleware`. Keep
  preflights ahead of authentication and method routing. Set `Vary: Origin` for
  every origin-dependent response, including disallowed origins; test both
  allowed and rejected preflights instead of copying the local implementation
  unchanged.
- **Request logging middleware** (`internal/api/middleware.go`): method, path,
  status, and duration through contextual `slog`, matching the gRPC interceptor.
  Capture the actual final status, including implicit 200s and repeated
  `WriteHeader` calls, and preserve streaming/connection interfaces on the
  wrapped writer. The local `statusRecorder` needs that care before extraction.
- **Development OIDC verifier** (`internal/api/accounts.go`): move the email-based
  `DevVerifier` into `auth/oidc` so applications using `HandleOIDCLogin` can run
  locally without provider credentials. Keep registration explicitly opt-in;
  preserve normalized, stable subjects and reject malformed email inputs.
- **Bounded JSON request decoding** (`internal/api/decks.go`, `cards.go`,
  `draws.go`, and `notes.go`): `http/server.Decode` reads the entire body with
  `io.ReadAll`; only artwork uploads currently have a body limit. Offer a
  bounded decoder or request middleware with a recognizable oversized-body
  error so handlers can return 413 consistently. Test the exact limit,
  overflow, and malformed JSON; keep domain validation in the app.
- **Bucket operations, if another app needs them** (`internal/media/storage.go`):
  configuration and client construction are shared, but `Put`, deletion that
  attempts every key, presigned GET URLs, and bucket health checks remain local.
  A small storage helper could own those operations and validate URL expiry.
  Keep cache headers and bucket creation explicit caller choices, and report a
  missing bucket as unhealthy. Avoid wrapping the entire AWS SDK.

Keep ent ownership queries, deck/draw rules, rendition names and sizes, card-stock
colors, the Wails configuration bridge, and browser caching/motion helpers in
Prayer Deck. They do not justify expanding the Go helper packages.
