# gitswitch - Need of the Hour (Atleast for me)

Switch your primary GitHub account — and everything git actually authenticates with.

`gsw switch work` and you are that account: commits attributed correctly, the
right SSH key offered, the right token used, the right signing key applied.

## Why

"GitHub identity" is not one setting. It is four independent layers, and tools
that flip only the first leave you pushing as the wrong account:

| Layer | Lives in | Symptom when wrong |
|---|---|---|
| Commit identity | `user.name` / `user.email` | Wrong attribution, no contribution graph |
| SSH auth | `~/.ssh/config`, ssh-agent | `Permission denied (publickey)` |
| HTTPS auth | credential helper, PAT | Prompts, or pushes as the wrong user |
| Commit signing | `user.signingkey`, `gpg.format` | Unverified badge, or signing fails |

gitswitch changes all four together, or none.

## Status

Early. Working today:

- `gsw add` — register a profile, generating an SSH key and printing it to add
- `gsw key` — re-print a profile's public key and the steps to register it
- `gsw list` — list profiles, active one marked
- `gsw edit` — change fields; only flags you pass are touched, `--root ""` clears
- `gsw switch` — change the active identity (also `gsw <profile>`)
- `gsw status` — active profile, plus the effective identity in this directory
- `gsw clone` — clone via the owning profile and pin identity into the new repo
- `gsw hook` — install a pre-push guard that blocks wrong-identity pushes
- `gsw ui` — local web UI for managing profiles and directory mappings
- `gsw remove` — delete a profile; the SSH key on disk is kept, and it says so
- `gsw restore` — remove every managed region: gitconfig, ssh config, and hook
- `gsw doctor` — local checks; network and keyring checks report as skipped

`internal/sshcfg` writes host aliases, generates keys, and verifies handshakes
against GitHub. Not built yet: `internal/keyring` (HTTPS tokens) and
`internal/github` (REST API); each carries a `doc.go` describing its intended
surface. Until they exist, SSH is the supported path and `gsw doctor` reports
token and signing-registration checks as skipped rather than passing.

[SCOPE.md](SCOPE.md) records what is in scope, what is deliberately not, the
known gaps in what ships today, and the order the rest gets built.

SSH is the supported path today. Key setup is deliberately manual — `gsw add`
generates the keypair and hands you the public key to paste, which avoids
requiring a registered OAuth app just to add an account.

## Install

```bash
go install github.com/sriyush/gitswitch/cmd/gsw@latest
```

Or from a clone:

```bash
make install      # builds to ./bin/gsw, installs to ~/.local/bin
```

Both need only Go — the web UI is embedded in the binary, so there is no node
toolchain, no asset directory, and nothing to install alongside it.

> **oh-my-zsh users:** the git plugin defines `alias gsw='git switch'`, which
> shadows this binary. Add `unalias gsw` to your `.zshrc` after the `oh-my-zsh.sh`
> line, or the command will silently run git's branch switcher instead.

## Use

```bash
# Generates an SSH key and prints it for you to add at github.com/settings/ssh/new
gsw add personal --username sriyush  --email you@personal.com  --root ~/personal
gsw add work     --username you-acme --email you@acme.com --root ~/work --orgs acme
gsw key work                  # show that key again later
gsw add ci --ssh-key ~/.ssh/existing_key --username you --email you@x.com

gsw switch work
gsw work                      # same thing
gsw clone acme/api            # resolves to the work profile via --orgs
gsw status
gsw doctor
gsw ui                        # manage everything from a local web page
```

## Directory-scoped identities

The feature that makes forgetting to switch harmless. A profile with a `--root`
gets an `includeIf` rule in `~/.gitconfig`:

```gitconfig
[includeIf "gitdir:/home/you/work/"]
	path = /home/you/.config/gitswitch/profiles/work.gitconfig
```

Git applies includes in order and later ones win, so inside `~/work` the work
identity overrides whatever is globally active. No daemon, no hook — git does it.

## Pre-push guard

Directory scoping prevents mistakes; the guard catches the ones that slip past.

```bash
gsw hook install
```

It blocks a push when a profile claims the repo's owner but you are committing
as someone else:

```
gitswitch: blocked push to Sriyush/GitSwitch

  This repo belongs to sriyush (Sriyush)
  You are committing as mahbrew (MahBrewski) <...@users.noreply.github.com>

  Fix:    gsw switch sriyush
  Or:     gsw edit mahbrew --orgs Sriyush   (if mahbrew really owns it)
  Bypass: GSW_SKIP_GUARD=1 git push
```

The check runs before git contacts the remote, and only fires when it is
confident: a profile must explicitly claim the owner via `--orgs` or a matching
username. Anything ambiguous is allowed, because a guard that cries wolf gets
uninstalled.

Installation sets `core.hooksPath`, which normally shadows every repo's local
hooks. The installed hook chains to `.git/hooks/pre-push` when one exists, so
husky and similar keep working. It refuses to install if `core.hooksPath` is
already set to something else.

## Web UI

```bash
gsw ui              # opens http://127.0.0.1:7842/?t=<session token>
gsw ui --no-open    # print the URL instead of launching a browser
gsw ui --port 9000  # when 7842 is taken; without a port it falls back to any free one
```

It runs in the foreground and prints the URL it opened. Ctrl-C stops it, and the
session token dies with the process.

You need the whole URL, `?t=…` included, the first time a tab opens it. After
that the tab keeps the token for as long as it stays open, so reloading is fine;
a fresh tab on a bare `127.0.0.1:7842` gets the page and an explanation, but no
data. If the browser did not open —
a headless box, an SSH session, or the wrong default browser — use `--no-open`
and paste the printed URL yourself. Over SSH, forward the port first:

```bash
ssh -L 7842:127.0.0.1:7842 you@box   # then run `gsw ui --no-open` on the box
```

Three screens, with the identity you are committing as pinned in the sidebar
throughout:

- **Accounts** — the active profile up top, the rest below it under "Switch to".
  Each carries a monogram coloured from its name, since two accounts look alike
  until you have already pushed with the wrong one. Inline editing, and the
  public key with a copy button next to a link to GitHub's key page.
- **Path mappings** — the main reason the UI exists, since editing `includeIf`
  rules by hand is miserable and a visual directory-to-profile map is not.
  Overlapping directories are refused, with the reason.
- **Doctor** — a verdict, a pass/fail/skip tally, and every check with its remedy
  inline, grouped per account. It runs on arrival; tick *skip network checks* to
  leave GitHub alone.

Paths are shown relative to `$HOME`, with the absolute path on hover. The page
subscribes to server-sent events, so running `gsw switch` in a terminal updates
an open tab.

An HTTP server that can rewrite `~/.gitconfig` is a serious thing to leave
running, so four rules are enforced server-side rather than left to the
frontend:

1. It binds `127.0.0.1` only, never `0.0.0.0`.
2. A random 256-bit session token is minted at startup and required as a bearer
   token on every API request. It lives in memory and dies with the process,
   which is why the UI only works from the URL `gsw ui` prints. The page moves it
   into `sessionStorage` and strips it from the address bar: scoped to that one
   tab and cleared when the tab closes, so reloading works but the token cannot
   outlive the window it was handed to. Restarting `gsw ui` invalidates it, and
   the page says so rather than showing a bare 401.
3. `Host` and `Origin` are validated on every API request. This is what stops
   DNS rebinding: a page that re-points its own hostname at 127.0.0.1 still
   sends its own `Origin`, and still gets a 403.
4. Token values are never returned by the API — only whether one is configured.

Every handler goes through the same `internal/profile` store and
`internal/apply` writer as the CLI, so the two cannot disagree about state.

## Safety

- Every edit to `~/.gitconfig` is fenced by `# >>> gitswitch managed >>>` markers.
  Content outside is preserved byte for byte, and `gsw restore` removes the block.
- A `.gsw-backup` is written before any modification.
- Config writes are atomic (temp file + rename), so an interrupted write cannot
  corrupt your `~/.gitconfig`.
- Tokens go to the OS keyring, never to disk. `profiles.json` holds a reference
  only, and is gitignored regardless.

## Layout

```
cmd/gsw/            entry point
internal/profile/   profile model + store — single source of truth
internal/apply/     writes the store out to gitconfig and ssh config
internal/gitcfg/    marker-fenced config editing + rendering
internal/sshcfg/    ~/.ssh/config, keygen, handshake verify
internal/checkup/   the checks behind `gsw doctor` and GET /api/doctor
internal/hook/      pre-push guard
internal/server/    localhost API behind `gsw ui`
web/                embedded frontend
  app/main.js         shell: tabs, event delegation, render loop
  app/lib/            api, store, SSE, DOM helpers
  app/features/       one folder per screen
internal/keyring/   OS credential store            (planned)
internal/github/    REST API                       (planned)
```

Zero third-party dependencies, and no npm: the frontend is hand-written and
embedded with `go:embed`, so `make build` needs nothing but Go and the result is
still a single binary. It is structured with native ES modules rather than a
bundler — see [web/README.md](web/README.md) for the module layout.

`internal/profile`, `internal/apply`, and `internal/checkup` are each shared by
the CLI and the server. That sharing is deliberate — a profile edited in the
browser and the same profile edited in a terminal must not be able to drift,
because the drift would be silent and would surface as a commit under the wrong
account.

## Develop

Go is not on this machine's `PATH`. A 1.22.5 toolchain is extracted at
`~/.local/go`; either add it or install a current Go (`sudo dnf install golang`):

```bash
export PATH=$HOME/.local/go/bin:$PATH
```

```bash
make test    # gitcfg has the meaningful coverage; it is the destructive path
make vet
make build
```

Test against a throwaway `HOME` so your real `~/.gitconfig` is never touched:

```bash
HOME=$(mktemp -d) ./bin/gsw add test --username me --email me@example.com
```

The same trick is the only safe way to develop the web UI, which writes real
config the moment you click anything:

```bash
d=$(mktemp -d)
HOME=$d XDG_CONFIG_HOME=$d/.config ./bin/gsw ui --port 7899
```

Frontend edits need a rebuild, because `go:embed` reads `web/app` at compile
time — `make build` and reload the page.

## Distribution

gitswitch is a local tool, not a service: there is no server to host and nothing
to deploy. Shipping it means putting a binary on someone else's machine, and it
is a single self-contained file with no runtime dependencies, which makes every
channel below straightforward.

**Go users, today, with no work from you.** Anyone with Go installed can already
run:

```bash
go install github.com/sriyush/gitswitch/cmd/gsw@latest
```

This works as soon as the repo is public and a version tag exists. Tag with
`git tag v0.1.0 && git push --tags`.

**Everyone else: GitHub Releases.** Cross-compiling is one command per target,
since cgo is not used:

```bash
GOOS=linux  GOARCH=amd64 go build -ldflags '-s -w -X main.version=v0.1.0' -o dist/gsw_linux_amd64  ./cmd/gsw
GOOS=darwin GOARCH=arm64 go build -ldflags '-s -w -X main.version=v0.1.0' -o dist/gsw_darwin_arm64 ./cmd/gsw
```

[GoReleaser](https://goreleaser.com) automates that — build matrix, checksums,
changelog, and the upload — from a `.goreleaser.yaml` plus a tag-triggered
workflow. Users then download one file, `chmod +x`, and move it onto `PATH`.

**Package managers**, once releases exist: a Homebrew tap is a formula pointing
at the release tarball (GoReleaser can push it for you), and Linux users get
`gsw` via the AUR or a Copr repo. Each is a wrapper around the same binary.

Two things to get right before publishing: the `unalias gsw` note above belongs
in your release notes, since oh-my-zsh users will otherwise report that the tool
"silently runs git", and `main.version` should be stamped at build time as the
`Makefile` already does, so `gsw --version` in a bug report means something.
