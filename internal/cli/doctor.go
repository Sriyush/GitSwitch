package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sriyush/gitswitch/internal/gitcfg"
	"github.com/sriyush/gitswitch/internal/profile"
	"github.com/sriyush/gitswitch/internal/sshcfg"
)

type checkResult struct {
	ok      bool
	label   string
	detail  string
	remedy  string
	skipped bool
}

// cmdDoctor validates the parts of the setup that can be checked locally.
//
// Live SSH handshakes, token scope/expiry checks, and "is this signing key
// registered on GitHub" all need network and keyring access, which land with the
// github and keyring packages. Those checks report as skipped rather than
// passing, so the output never overstates what was verified.
func cmdDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	offlineFlag := fs.Bool("offline", false, "skip checks that contact GitHub")
	if err := fs.Parse(args); err != nil {
		return err
	}
	offline := *offlineFlag

	store, err := profile.Load()
	if err != nil {
		return err
	}
	list := store.List()
	if len(list) == 0 {
		fmt.Println("No profiles configured. Run `gsw add <name> --username <login> --email <email>`.")
		return nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir, err := profile.ConfigDir()
	if err != nil {
		return err
	}

	var failed int
	for _, p := range list {
		fmt.Printf("\n%s (%s)\n", p.Name, p.Username)

		var checks []checkResult
		checks = append(checks, checkFile(
			"commit identity fragment",
			filepath.Join(dir, "profiles", p.Name+".gitconfig"),
			"run `gsw switch "+p.Name+"` to regenerate",
		))

		if p.SSHKey == "" {
			checks = append(checks, checkResult{
				label:  "ssh key",
				detail: "not configured",
				remedy: "gsw edit " + p.Name + " --ssh-key ~/.ssh/id_ed25519",
			})
		} else {
			checks = append(checks, checkFile("ssh key", p.SSHKey, "generate one with `ssh-keygen -t ed25519`"))
		}

		if p.Root != "" {
			checks = append(checks, checkDir("scoped root", p.Root, "create it or clear it with `gsw edit "+p.Name+" --root \"\"`"))
		}

		// The handshake is the check that actually matters: it proves GitHub
		// resolves this profile's key to the account the profile claims.
		if p.SSHKey == "" {
			checks = append(checks, checkResult{label: "ssh handshake", skipped: true, detail: "no key configured"})
		} else if offline {
			checks = append(checks, checkResult{label: "ssh handshake", skipped: true, detail: "skipped (--offline)"})
		} else {
			alias := p.DefaultHostAlias()
			switch login, err := sshcfg.Verify(alias); {
			case err != nil:
				checks = append(checks, checkResult{
					label:  "ssh handshake",
					detail: fmt.Sprintf("%s: %v", alias, err),
					remedy: "add the public key at https://github.com/settings/keys for " + p.Username,
				})
			case !strings.EqualFold(login, p.Username):
				checks = append(checks, checkResult{
					label:  "ssh handshake",
					detail: fmt.Sprintf("%s authenticates as %q, expected %q", alias, login, p.Username),
					remedy: "this key belongs to a different account; generate a separate key for " + p.Username,
				})
			default:
				checks = append(checks, checkResult{ok: true, label: "ssh handshake", detail: "authenticated as " + login})
			}
		}

		checks = append(checks,
			checkResult{label: "token validity", skipped: true, detail: "needs internal/keyring"},
			checkResult{label: "signing key on GitHub", skipped: true, detail: "needs internal/github"},
		)

		for _, c := range checks {
			switch {
			case c.skipped:
				fmt.Printf("  ~ %-24s %s\n", c.label, c.detail)
			case c.ok:
				fmt.Printf("  + %-24s %s\n", c.label, c.detail)
			default:
				failed++
				fmt.Printf("  x %-24s %s\n", c.label, c.detail)
				if c.remedy != "" {
					fmt.Printf("    -> %s\n", c.remedy)
				}
			}
		}
	}

	fmt.Println()
	if _, ok, err := gitcfg.Block(filepath.Join(home, ".gitconfig")); err != nil {
		return err
	} else if !ok {
		failed++
		fmt.Println("x ~/.gitconfig has no gitswitch block")
		fmt.Println("  -> run `gsw switch <profile>` to write it")
	} else {
		fmt.Println("+ ~/.gitconfig contains the gitswitch block")
	}

	if failed > 0 {
		return fmt.Errorf("%d check(s) failed", failed)
	}
	fmt.Println("\nAll local checks passed.")
	return nil
}

func checkFile(label, path, remedy string) checkResult {
	fi, err := os.Stat(path)
	if err != nil {
		return checkResult{label: label, detail: "missing: " + path, remedy: remedy}
	}
	if fi.IsDir() {
		return checkResult{label: label, detail: "expected a file: " + path, remedy: remedy}
	}
	return checkResult{ok: true, label: label, detail: path}
}

func checkDir(label, path, remedy string) checkResult {
	fi, err := os.Stat(path)
	if err != nil {
		return checkResult{label: label, detail: "missing: " + path, remedy: remedy}
	}
	if !fi.IsDir() {
		return checkResult{label: label, detail: "not a directory: " + path, remedy: remedy}
	}
	return checkResult{ok: true, label: label, detail: path}
}

func cmdUI(args []string) error {
	return fmt.Errorf("not implemented yet - internal/server and web/ are scaffolded but empty")
}
