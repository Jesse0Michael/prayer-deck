# Prayer Deck client

The Wails v3 client: a React + TypeScript frontend (`frontend/`) and a small Go
shell (`main.go`) that serves it.

- **Web:** `go build -tags server` serves the frontend over HTTP to any browser.
  This is what `make up` runs on http://localhost:3000.
- **iOS (experimental upstream):** `wails3 task ios:run` on macOS with Xcode.
  See `../docs/decisions.md` before going further.

The API URL comes from `PRAYER_DECK_API_URL` at runtime through `ConfigService`
(builds without an environment, such as iOS, set it with
`-ldflags "-X main.defaultAPIURL=..."`).

```bash
cd frontend
npm run dev            # hot reload at http://127.0.0.1:9245 (expects the API on :8080)
npm run generate:api   # regenerate src/api/schema.d.ts from ../../api/openapi.yaml
npm run typecheck
```
