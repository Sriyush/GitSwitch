// Package checkup runs the diagnostics behind `gsw doctor` and GET /api/doctor.
//
// The checks return data rather than printed lines so the CLI and the web UI can
// render the same result differently without either re-implementing what "a
// healthy profile" means.
//
// A check that has not been implemented reports StatusSkip, never StatusPass.
// Overstating what was verified is the one thing a diagnostic must not do: a
// green line next to "token validity" that checked nothing is worse than no line
// at all, because it is believed.
package checkup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sriyush/gitswitch/internal/gitcfg"
	"github.com/sriyush/gitswitch/internal/profile"
	"github.com/sriyush/gitswitch/internal/sshcfg"
)

// Status is the outcome of a single check.
type Status string

const (
	StatusPass Status = "pass"
	StatusFail Status = "fail"
	StatusSkip Status = "skip"
)

// Check is one diagnostic and, when it fails, the command that fixes it.
type Check struct {
	Label  string `json:"label"`
	Status Status `json:"status"`
	Detail string `json:"detail"`
	Remedy string `json:"remedy,omitempty"`
}

// Report is every check for one profile.
type Report struct {
	Profile  string  `json:"profile"`
	Username string  `json:"username"`
	Checks   []Check `json:"checks"`
}

// Result is a full run: per-profile reports plus checks that are not tied to any
// one profile.
type Result struct {
	Profiles []Report `json:"profiles"`
	Global   []Check  `json:"global"`
	Failed   int      `json:"failed"`
}

// Run executes every check that can be performed locally.
//
// offline skips the SSH handshake, which is the only check that touches the
// network. Token scope and signing-key registration need internal/keyring and
// internal/github respectively, and report as skipped until those exist.
func Run(store *profile.Store, offline bool) (*Result, error) {
	dir, err := profile.ConfigDir()
	if err != nil {
		return nil, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	res := &Result{}
	for _, p := range store.List() {
		rep := Report{Profile: p.Name, Username: p.Username}

		rep.Checks = append(rep.Checks, checkFile(
			"commit identity fragment",
			filepath.Join(dir, "profiles", p.Name+".gitconfig"),
			"run `gsw switch "+p.Name+"` to regenerate",
		))

		if p.SSHKey == "" {
			rep.Checks = append(rep.Checks, Check{
				Label:  "ssh key",
				Status: StatusFail,
				Detail: "not configured",
				Remedy: "gsw edit " + p.Name + " --ssh-key ~/.ssh/id_ed25519",
			})
		} else {
			rep.Checks = append(rep.Checks, checkFile("ssh key", p.SSHKey, "generate one with `ssh-keygen -t ed25519`"))
		}

		if p.Root != "" {
			rep.Checks = append(rep.Checks, checkDir("scoped root", p.Root,
				"create it or clear it with `gsw edit "+p.Name+" --root \"\"`"))
		}

		rep.Checks = append(rep.Checks, sshHandshake(p, offline))

		rep.Checks = append(rep.Checks,
			Check{Label: "token validity", Status: StatusSkip, Detail: "needs internal/keyring"},
			Check{Label: "signing key on GitHub", Status: StatusSkip, Detail: "needs internal/github"},
		)

		res.Profiles = append(res.Profiles, rep)
	}

	gitconfigPath := filepath.Join(home, ".gitconfig")
	switch _, ok, err := gitcfg.Block(gitconfigPath); {
	case err != nil:
		return nil, err
	case ok:
		res.Global = append(res.Global, Check{
			Label:  "gitconfig block",
			Status: StatusPass,
			Detail: gitconfigPath + " contains the gitswitch block",
		})
	default:
		res.Global = append(res.Global, Check{
			Label:  "gitconfig block",
			Status: StatusFail,
			Detail: gitconfigPath + " has no gitswitch block",
			Remedy: "run `gsw switch <profile>` to write it",
		})
	}

	for _, rep := range res.Profiles {
		for _, c := range rep.Checks {
			if c.Status == StatusFail {
				res.Failed++
			}
		}
	}
	for _, c := range res.Global {
		if c.Status == StatusFail {
			res.Failed++
		}
	}
	return res, nil
}

// sshHandshake is the check that actually matters: it proves GitHub resolves
// this profile's key to the account the profile claims. Everything else confirms
// that files exist.
func sshHandshake(p *profile.Profile, offline bool) Check {
	switch {
	case p.SSHKey == "":
		return Check{Label: "ssh handshake", Status: StatusSkip, Detail: "no key configured"}
	case offline:
		return Check{Label: "ssh handshake", Status: StatusSkip, Detail: "skipped (offline)"}
	}

	alias := p.DefaultHostAlias()
	login, err := sshcfg.Verify(alias)
	switch {
	case err != nil:
		return Check{
			Label:  "ssh handshake",
			Status: StatusFail,
			Detail: fmt.Sprintf("%s: %v", alias, err),
			Remedy: "add the public key at https://github.com/settings/keys for " + p.Username,
		}
	case !strings.EqualFold(login, p.Username):
		return Check{
			Label:  "ssh handshake",
			Status: StatusFail,
			Detail: fmt.Sprintf("%s authenticates as %q, expected %q", alias, login, p.Username),
			Remedy: "this key belongs to a different account; generate a separate key for " + p.Username,
		}
	default:
		return Check{Label: "ssh handshake", Status: StatusPass, Detail: "authenticated as " + login}
	}
}

func checkFile(label, path, remedy string) Check {
	fi, err := os.Stat(path)
	if err != nil {
		return Check{Label: label, Status: StatusFail, Detail: "missing: " + path, Remedy: remedy}
	}
	if fi.IsDir() {
		return Check{Label: label, Status: StatusFail, Detail: "expected a file: " + path, Remedy: remedy}
	}
	return Check{Label: label, Status: StatusPass, Detail: path}
}

func checkDir(label, path, remedy string) Check {
	fi, err := os.Stat(path)
	if err != nil {
		return Check{Label: label, Status: StatusFail, Detail: "missing: " + path, Remedy: remedy}
	}
	if !fi.IsDir() {
		return Check{Label: label, Status: StatusFail, Detail: "not a directory: " + path, Remedy: remedy}
	}
	return Check{Label: label, Status: StatusPass, Detail: path}
}
