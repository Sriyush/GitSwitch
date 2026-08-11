# gitswitch — scope and roadmap

Last reviewed: 2026-08-10 · Status: v0.2, SSH path and web UI working, HTTPS unbuilt

This document exists because the README describes what gitswitch *is*, and the
`doc.go` files describe what individual packages *will be*, but nothing recorded
what is deliberately in scope, what is deliberately not, and in what order the
unbuilt parts get built. This is that record.

---

## 1. Problem

"GitHub identity" is not one setting. It is four independent layers, and every
tool that flips only the first leaves the user pushing as the wrong account:

| Layer | Lives in | Symptom when wrong |
|---|---|---|
| Commit identity | `user.name` / `user.email` | Wrong attribution, no contribution graph |
| SSH auth | `~/.ssh/config`, ssh-agent | `Permission denied (publickey)` |
| HTTPS auth | credential helper, PAT | Prompts, or pushes as the wrong user |
| Commit signing | `user.signingkey`, `gpg.format` | Unverified badge, or signing fails |

The failure mode that matters is the *silent* one: the push succeeds, and the
commit lands under the wrong account. Everything in this document is ranked by
how much it reduces silent wrong-identity pushes.

## 2. What gitswitch is

A single Go binary (`gsw`) that changes all four layers together, or none, and
that makes forgetting to switch harmless.

**Product commitment:** after `gsw switch work`, every one of the four layers
authenticates as `work` — and inside a directory owned by another profile, git
overrides that automatically with no daemon running.

## 3. Design invariants

These are the constraints that decide scope arguments. Changing one of these is
a bigger decision than adding any feature below.

1. **Never corrupt user config.** Every write to a file the user owns is
   marker-fenced, backed up to `.gsw-backup`, and atomic (temp + rename).
   Content outside the fence survives byte for byte.
2. **State lives in one place.** `internal/profile` is the single source of
   truth. The CLI and the future server both go through it, so they cannot
   disagree. No feature gets its own parallel store.
3. **No daemon.** Directory scoping is `includeIf` in `~/.gitconfig`; git itself
   does the work. A background process that has to be running for identity to be
   correct is a regression, not a feature.
4. **Secrets never touch disk.** Tokens go to the OS keyring. `profiles.json`
   holds a `keyring://` reference only.
5. **The guard never cries wolf.** The pre-push check blocks only when a profile
   explicitly claims the repo owner *and* the committing identity differs.
   Ambiguity is allowed through. A guard with false positives gets uninstalled,
   and then it protects nothing.
6. **Dependencies are earned, not assumed.** Zero third-party modules so far,
   Go or npm. The bar for the first one is: the stdlib alternative is materially
   worse (cross-platform keyring is the expected first exception). This is why
   the frontend is hand-written — requiring a node toolchain to compile a Go
   binary is a cost paid by everyone who builds the project, in exchange for
   convenience on four screens.
7. **Never claim more verification than was performed.** `gsw doctor` reports
   unimplemented checks as *skipped*, not as passing.

## 4. In scope

- GitHub accounts on a single developer workstation.
- The four identity layers above, plus the directory-scoping and pre-push guard
  that make them hard to get wrong.
- Local key material lifecycle: generate a keypair per profile, show it for
  registration, later upload it via the API.
- A local-only web UI for the parts that are genuinely miserable in a terminal
  (path mappings, primarily).
- Linux first (the dev machine is Fedora), macOS and Windows as they come.

## 5. Out of scope (and why)

| Not doing | Why |
|---|---|
| GitLab / Bitbucket / Gitea | Every layer is provider-shaped: host aliases, API endpoints, key registration, `X-OAuth-Scopes`. Multi-provider is a rewrite of the model, not a flag. Revisit only if the GitHub path is finished and someone actually asks. |
| Team / fleet management, config sync | This is a personal workstation tool. Anything that syncs identity config across machines invites secret material into a transport, which fights invariant 4. |
| A hosted service | The server is localhost-only by design (invariant: bind `127.0.0.1`, never `0.0.0.0`). There is no product here that wants a backend. |
| Managing repo contents, branches, or PRs | `gh` exists and is better at it. gitswitch owns identity, not workflow. |
| Automatic identity detection by heuristic (guessing from remote, README, etc.) | Guessing produces exactly the silent-wrong-push failure this tool exists to prevent. Identity is declared (`--root`, `--orgs`) and enforced. |
| A shell prompt integration / daemon that watches `cd` | Invariant 3. `includeIf` already gives correct behaviour with nothing running. |
| Storing tokens in `profiles.json` "for convenience" | Invariant 4, non-negotiable. |
| Rewriting history to fix past mis-attributed commits | Destructive, and a different problem. Point at `git filter-repo` in docs at most. |

## 6. Current state (v0.2)

### Working

| Command | Behaviour |
|---|---|
| `gsw add` | Registers a profile, generates an ed25519 key (one per account — GitHub rejects a key already registered elsewhere), prints it for registration |
| `gsw key` | Re-prints a profile's public key and the registration steps |
| `gsw list` / `ls` | Lists profiles, active one marked |
| `gsw edit` | Changes fields in place; only flags passed are touched, `--root ""` clears |
| `gsw switch <p>` / `gsw <p>` | Rewrites all managed config regions from the store |
| `gsw status` | Active profile plus the *effective* identity in the current directory |
| `gsw clone owner/repo` | Resolves the owning profile via `--orgs`, clones through the host alias, pins identity into the new repo's local config |
| `gsw hook install` | Pre-push guard; chains to `.git/hooks/pre-push` so husky et al. keep working |
| `gsw ui` | Local web UI: profiles, path mappings, doctor. Token-authenticated, loopback-only |
| `gsw remove` | Deletes a profile; keeps the SSH key on disk and says so |
| `gsw restore` | Removes every managed region: gitconfig block, ssh config block, `core.hooksPath` |
| `gsw doctor` | Local checks + live SSH handshake; unimplemented checks report as skipped |

Load-bearing details worth not regressing: `IdentitiesOnly yes` in the SSH block
(without it ssh offers every agent key and GitHub picks whichever matches first —
the single most common silent wrong-account push); the absolute `gsw` path baked
into the hook script (git's hook PATH does not include `~/.local/bin`, so a bare
`gsw` would silently no-op); and `splitRepo` parsing by URL structure rather than
matching the literal `github.com`, since gitswitch's own remotes carry host
aliases like `github.com-work`.

The web UI is served from `internal/server` and `web/`, with the profile store,
the config writer (`internal/apply`), and the diagnostics (`internal/checkup`)
shared with the CLI so the two cannot drift.

### Not built

`internal/keyring` and `internal/github` — each has a `doc.go` stating its
intended surface, and nothing else. HTTPS is therefore unsupported: SSH is the
only working path, and the UI reports token status as "unsupported" rather than
"none", since the latter would imply a working feature with nothing in it.

### Known gaps in what ships today

Ordered by user impact. These are debts of the current code, not roadmap items.

1. **The guard keys on `user.email` only.** A profile whose email matches but
   whose SSH key belongs to another account passes. Comparing the key actually
   offered to the remote would close it.
2. **`gsw add` never verifies the key was registered.** It prints the key and
   assumes; `gsw doctor` is the only thing that catches "you never pasted it".
3. **Signing is accepted but unverified.** `--signing-key` is written into config
   and pinned on clone, but nothing generates a signing key or confirms GitHub
   knows it.
4. **The UI has never been rendered in a real browser here.** Its endpoints,
   payload fields, and security rules are covered by tests, and the assets are
   served and parse cleanly — but headless Firefox could not allocate a
   framebuffer on this machine, so nobody has yet looked at the page. Run
   `gsw ui` before trusting the layout.
5. **`internal/hook` still has no tests.** The install/uninstall path touches
   global git config, which makes it the remaining untested destructive surface.
6. **Nothing debounces the store watcher.** `gsw ui` polls `profiles.json` once a
   second to notice terminal-side changes; a script switching profiles in a tight
   loop would produce one SSE event per second rather than one per change. Not
   worth solving until someone hits it.

## 7. Roadmap

Milestones were ordered by the invariant they serve: close the silent-failure
surface first, then the manual toil, then the UI. M1 and M5 are done; the
remaining order is **M2 → M3 → M4 → M6**, which is unchanged — HTTPS support is
the largest hole left, and M3 depends on M2 for somewhere to keep a token.

### M1 — Correctness debts — **done (v0.2)**

The guard can no longer be defeated by capitalisation, and uninstall is clean.

- Owner resolution is case-insensitive (`Store.ResolveOwner`), which also closed
  the hole where the pre-push guard silently allowed `Acme/api` against a profile
  scoped to `acme`.
- `gsw restore` clears all three managed regions, and reports only the ones that
  were actually present.
- Stale identity fragments are pruned on every apply; `gsw remove` states that
  the SSH key was kept.
- Overlapping `--root` values are refused at `add`, `edit`, and over the API,
  with the reason attached to the error rather than left to the reader.
- Tests added for `ResolveOwner`, `CheckRoot`, `sshcfg.Render`, the SSH banner
  parser, the apply/prune path, and the server's auth and Origin rules.

### M2 — `internal/keyring`: HTTPS tokens

**Goal:** HTTPS stops being unsupported.

- `Set(ref, token)` / `Get(ref)` / `Delete(ref)`, backed by libsecret via D-Bus
  on Linux, Keychain on macOS, Credential Manager on Windows.
- `Configure(profile)` — point git's credential helper at the profile's token.
- `gsw token set|clear <profile>`, reading from stdin so the token never reaches
  the shell history.
- Doctor's "token validity" check stops reporting *skipped* for presence (scope
  and expiry still need M3), in both the CLI and the UI — they render the same
  `internal/checkup` result, so the check only has to be written once.
- The UI's `token_status` field stops being the constant `"unsupported"` and
  starts reporting real presence. It must still never carry the value itself.

**Dependency call:** `github.com/zalando/go-keyring` covers all three platforms
without cgo. This is the intended first exception to invariant 6 — reimplementing
three OS credential APIs is materially worse than one well-scoped dependency.

**Risk:** a headless Linux box with no unlocked keyring has nowhere to put a
token. Fail with a clear message; do not silently fall back to a file.

### M3 — `internal/github`: REST API

**Goal:** stop asking the user to paste things into a browser.

- `DeviceFlow()` — OAuth device authorization, so `gsw add` never needs a PAT
  pasted by hand.
- `UploadSSHKey(token, pubkey)` and `UploadSigningKey(token, pubkey)` — collapse
  the current print-and-paste step into one confirmation.
- `WhoAmI(token)` — confirm a token belongs to the account its profile claims.
- `TokenScopes(token)` — read `X-OAuth-Scopes` and the expiry header so doctor
  warns *before* a fine-grained PAT lapses.
- Doctor's remaining skipped checks (token validity, signing key on GitHub) go
  live; `--offline` remains the escape hatch.

**Open question:** device flow needs a registered OAuth app client ID shipped in
the binary. Until one exists, `gsw add` keeps the manual paste path — which is
why key setup is deliberately manual today. Registering the app is a prerequisite
for this milestone, not part of it.

**Constraint:** stdlib `net/http` only. This is a handful of endpoints; an SDK is
not warranted.

### M4 — Signing, end to end

**Goal:** the fourth layer gets the same treatment as the other three.

- Generate an SSH signing key on request, or reuse the auth key where the user
  prefers it.
- Upload it as a *signing* key via M3 and verify the Verified badge appears.
- Doctor check: signing key present locally, registered remotely, and matching
  the profile's account.

### M5 — `internal/server` + `web/`: local UI — **done (v0.2)**

Built ahead of M2–M4 because it was asked for. Path mappings are now editable by
a human, which was the point: editing `includeIf` rules by hand is the worst part
of the tool, and a visual directory-to-profile map is not.

All four security rules are enforced server-side and covered by tests: loopback-
only bind, 256-bit in-memory bearer token, `Host` + `Origin` validation against
DNS rebinding, and no token values on the wire.

Routes: `GET/POST /api/profiles`, `PATCH/DELETE /api/profiles/{name}`,
`GET /api/profiles/{name}/key`, `POST /api/switch`, `GET /api/doctor`,
`GET /api/events` (SSE, so an open tab stays correct when the identity is
switched from a terminal).

Frontend: hand-written HTML/CSS/JS under `web/app`, embedded via `go:embed`.
This is a change from the original plan of React + Vite + Tailwind — see
invariant 6. Four screens over a small JSON API did not justify making a node
toolchain a prerequisite for building a Go binary. `web/embed.go` is where a
generated `dist/` would go if the UI ever outgrows that trade.

Screens: Profiles (list, switch, add, edit, remove, public key) · **Path
mappings** · Doctor. "Add account" via OAuth device flow still waits on M3;
until then the UI generates a key and shows it for manual registration, exactly
as the CLI does.

All handlers delegate to `internal/profile`, `internal/apply`, and
`internal/checkup` — invariant 2.

### M6 — Distribution

**Goal:** installable by someone who is not the author.

- Tagged releases with cross-compiled binaries (Linux/macOS/Windows, amd64/arm64).
- Homebrew tap and an AUR or COPR package.
- Shell completions. This is the point at which swapping the hand-rolled command
  table for cobra pays for itself; `internal/cli` was written so nothing outside
  `cli.go` assumes either.
- `gsw import` — adopt an existing `~/.gitconfig` identity as a profile, so the
  first run is not "start over".

## 8. Cross-cutting concerns

**Portability.** Everything today assumes POSIX paths, `~/.ssh/config`, and a
`/bin/sh` hook script. Windows needs a separate hook script and a different
credential path. Decide at M2 whether Windows is supported or explicitly
unsupported — silently half-working is the bad outcome. (`gsw ui` already opens
the right browser per platform, which is the cheap part.)

**Testing.** `gitcfg`, `sshcfg`, `profile`, `apply`, and `server` now have
coverage; `hook` is the one destructive path still untested. A config-corrupting
bug costs more trust than every feature above adds. All tests run against a
throwaway `HOME` set with `t.Setenv`, so a test can never reach the developer's
real `~/.gitconfig`.

**Toolchain.** Go is not on this machine's `PATH`; a 1.22.5 toolchain sits at
`~/.local/go`. `go.mod` declares 1.22. Worth pinning in CI when CI exists.

## 9. Decisions still open

1. **OAuth app registration** — blocks M3's device flow. Who owns the app, and
   does the client ID ship in the binary?
2. **Windows: supported or explicitly not?** Cheapest to answer before M2.
3. **cobra at M6, or keep the hand-rolled table?** Completions are the deciding
   factor.
4. **Does the guard extend to `pre-commit`?** Blocking at push is late but
   cheap; blocking at commit is earlier but noisier and fights invariant 5.
5. **Does `gsw remove` ever delete key material?** Currently no — it now says so
   in both the CLI and the UI. Should it offer to?
6. **Does the UI ever get to run without `gsw ui` in the foreground?** Today the
   server dies with the terminal that started it, which is what makes the
   in-memory token honest. A background mode would need somewhere to keep the
   token, and that is a real trade rather than a convenience.
