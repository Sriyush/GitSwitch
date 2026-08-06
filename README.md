# gitswitch

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
- `gsw remove`, `gsw restore`
- `gsw doctor` — local checks only; network and keyring checks report as skipped

`internal/sshcfg` writes host aliases, generates keys, and verifies handshakes
against GitHub. Not built yet: `internal/keyring` (HTTPS tokens), `internal/github`
(REST API), `internal/server`, and the web UI. Each carries a `doc.go` describing
its intended surface.

SSH is the supported path today. Key setup is deliberately manual — `gsw add`
generates the keypair and hands you the public key to paste, which avoids
requiring a registered OAuth app just to add an account.

## Install

```bash
make install      # builds to ./bin/gsw, installs to ~/.local/bin
```

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
internal/gitcfg/    marker-fenced config editing + rendering
internal/sshcfg/    ~/.ssh/config + agent          (planned)
internal/keyring/   OS credential store            (planned)
internal/github/    REST API + OAuth device flow   (planned)
internal/server/    localhost API behind `gsw ui`  (planned)
web/                React + Vite frontend          (planned)
```

Zero third-party dependencies so far. `internal/profile` is shared by the CLI and
the future server, so the two cannot disagree about state.

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

<!-- push test -->
<!-- mahbrew access test -->
