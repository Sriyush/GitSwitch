# gitswitch web UI

The frontend served by `gsw ui`. It is embedded into the binary with `go:embed`
(see `embed.go`), so an installed `gsw` remains a single file.

## No build step

`app/` is hand-written HTML, CSS, and JavaScript with no framework, no bundler,
and no npm. That is a deliberate trade: the UI is three screens over a small JSON
API, which a framework would not make meaningfully simpler, and requiring a node
toolchain to compile a Go binary is a cost paid by everyone who ever builds the
project.

Edit the files and rebuild — `make build` picks them up, because `go:embed` reads
them at compile time. There is nothing to watch, install, or transpile. If the UI
ever outgrows this, `embed.go` is where a generated `dist/` would be embedded
instead.

## Layout

Structure comes from native ES modules rather than from a bundler. The browser
resolves the imports itself, so the layout is feature-based without a build
step in front of it:

```
app/
  index.html            app shell: sidebar, banner, #view
  main.js               nav, event delegation, render loop
  style.css             tokens, then components
  lib/
    api.js              session token + fetch wrapper
    events.js           SSE subscription
    store.js            all client state, and refresh()
    dom.js              $, esc, banner, guard, busy
    paths.js            ~-folding for display
    icons.js            inline 16px stroke icons
    avatar.js           per-profile monogram
  features/
    profiles/index.js   list, switch, add, edit, remove, public key
             card.js    one profile card + the key block
             forms.js   add and edit forms
    paths/index.js      directory-to-profile mappings
    doctor/index.js     checks, tally, remedies
```

The sidebar nav is generated from the feature list, so a new screen never means
editing `index.html` as well.

Every feature module default-exports the same shape:

```js
export default {
  id: 'profiles',              // matches data-view on its tab button
  label: 'Profiles',
  render() { return '<html>' },// returns the innerHTML for #view
  actions: { switch(ctx) {} }, // keyed by data-act on a button
  forms: { 'add-form'(ctx) {} },// keyed by form id or class
  onEnter() {},                // optional, runs when the tab is opened
};
```

`main.js` holds one click listener and one submit listener for the whole
document and dispatches by `data-act` into the current feature's `actions`. That
delegation is what lets `render()` replace the entire view with `innerHTML`
without ever rebinding a handler.

Dependencies point one way: features import from `lib/`, `lib/store.js` imports
nothing of theirs, and `main.js` is the only module that knows both. Adding a
screen is adding a folder and one line in `FEATURES`.

## State

`lib/store.js` holds one object. Any feature can call `refresh()` after a
mutation and every other screen comes back correct, because there is only one
copy of the truth and `notify()` re-renders whatever tab is open.

Server-sent events call the same `refresh()`, so `gsw switch` in a terminal
updates an open tab.

## Auth

The server binds `127.0.0.1` only, mints a random 256-bit session token at
startup, and opens `http://127.0.0.1:7842/?t=<token>`.

The app reads `t` from the query string, moves it into `sessionStorage`, drops it
from the URL via `history.replaceState`, and sends it as
`Authorization: Bearer <token>` on every request.

`sessionStorage` and never `localStorage`. The difference is the whole point:
`sessionStorage` is scoped to the one tab and cleared when that tab closes, so a
reload keeps working while the token cannot outlive the window it was handed to.
`localStorage` would leave a dead credential in the browser profile long after
the process it authenticates against had exited.

A token in the URL always wins over a stored one, since it came from the `gsw ui`
running now. Any 401 discards the stored copy and says so in plain words — a
stale token means `gsw ui` was restarted, and the fix is to open the URL it
printed, not to debug an authentication error.

`GET /api/events` is the exception: `EventSource` cannot set headers, so it
authenticates by query parameter. The token is no more exposed there than in the
URL it came from, and the connection never leaves the loopback interface.

The server also validates `Host` and `Origin` on every API request, which is what
blocks DNS rebinding. A page opened without a token is not broken — it says so,
and points at the URL `gsw ui` prints.

## Design

The page is an app shell: a sidebar holding the nav and, pinned at its foot, the
identity you are currently committing as. That question is the one the whole tool
exists to answer, so it gets a permanent home rather than a line that scrolls
away.

Each profile carries a monogram whose colour is derived from its name — stable
across machines and restarts, with nothing stored. Two accounts look alike until
you have already pushed with the wrong one, and a colour tells them apart faster
than reading `you@acme.com` in 12px monospace.

Colours, radii, and shadows are custom properties at the top of `style.css`, with
the dark scheme redefining only the tokens. Nothing below the token block hard-codes
a colour, so the two schemes cannot drift.

## Screens

1. **Accounts** — the active profile rendered large at the top, the rest listed
   below it under "Switch to": same component, different emphasis. Editing is
   inline, adding is behind a disclosure, and the public key comes with a copy
   button next to a link to GitHub's key page.
2. **Path mappings** — the main reason the UI exists; editing `includeIf` rules
   by hand is miserable, and a visual directory-to-profile mapping is not.
   Overlapping directories are refused by the server, since nested scopes would
   make the winning identity depend on config order.
3. **Doctor** — a verdict line and a pass/fail/skip tally, then the checks
   grouped per account with each remedy inline. It runs on arrival, since a
   doctor screen with nothing in it is a dead end. Skipped is rendered
   distinctly from passed, because the checks that need `internal/keyring` and
   `internal/github` verify nothing yet.

Paths are displayed relative to `$HOME` (`~/work`, not `/home/you/work`) with the
absolute path in the `title` attribute. They are *stored* absolute — git and ssh
read them literally, with no shell to expand a tilde — so the folding is display
only, and every form field still carries the real value.

## Not here yet

Adding an account still means pasting a generated public key into GitHub
yourself. The OAuth device flow that would replace it needs `internal/github`
and a registered OAuth app — see [SCOPE.md](../SCOPE.md), M3.
