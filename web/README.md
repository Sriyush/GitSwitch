# gitswitch web UI

Frontend for the local management server (`gsw ui`). Not scaffolded yet.

## Planned setup

```bash
npm create vite@latest . -- --template react-ts
npm install -D tailwindcss @tailwindcss/vite
npm run build     # emits ./dist, embedded into the binary via go:embed
```

Vite must emit to `web/dist`, since `internal/server` embeds that path. Set
`base: './'` in `vite.config.ts` so assets resolve when served from the binary.

## Auth

The server mints a random session token at startup and opens
`http://127.0.0.1:7842/?t=<token>`. The app should read `t` from the query
string, drop it from the URL via `history.replaceState`, hold it in memory only
(never `localStorage` — that survives the process the token is scoped to), and
send it as `Authorization: Bearer <token>` on every request.

## Screens

1. **Profiles** — list, active indicator, one-click switch.
2. **Profile detail** — identity, SSH key, token status, signing key.
3. **Path mappings** — the main reason the UI exists; editing `includeIf` rules
   by hand is miserable, and a visual directory-to-profile mapping is not.
4. **Doctor** — per-check pass/fail with the remedy inline.
5. **Add account** — OAuth device flow, which wants a browser regardless.

Subscribe to `GET /api/events` (SSE) so the UI stays correct when the identity is
switched from a terminal.
