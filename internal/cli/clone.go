package cli

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sriyush/gitswitch/internal/profile"
)

// cmdClone clones a repo under the profile that owns it, pinning the identity
// into the new repo's local config. Pinning matters because a repo cloned into
// the wrong tree would otherwise silently inherit whichever profile is active
// later on.
func cmdClone(args []string) error {
	fs := flag.NewFlagSet("clone", flag.ContinueOnError)
	as := fs.String("as", "", "force a specific profile instead of resolving by owner")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: gsw clone <owner/repo> [destination] [--as <profile>]")
		fs.PrintDefaults()
	}
	pos, flags := splitPositional(args)
	if err := fs.Parse(flags); err != nil {
		return err
	}
	pos = append(pos, fs.Args()...)
	if len(pos) < 1 {
		fs.Usage()
		return fmt.Errorf("expected <owner/repo>")
	}

	owner, repo, ok := splitRepo(pos[0])
	if !ok {
		return fmt.Errorf("expected <owner/repo>, got %q", pos[0])
	}

	store, err := profile.Load()
	if err != nil {
		return err
	}

	var p *profile.Profile
	switch {
	case *as != "":
		if p, err = store.Get(*as); err != nil {
			return err
		}
	default:
		var found bool
		if p, found = store.ResolveOwner(owner); !found {
			active, ok := store.ActiveProfile()
			if !ok {
				return fmt.Errorf("no profile owns %q and no profile is active; retry with --as <profile>", owner)
			}
			p = active
			fmt.Printf("No profile claims %q; using active profile %q.\n", owner, p.Name)
			fmt.Printf("Bind it permanently with: gsw edit %s --orgs %s\n\n", p.Name, owner)
		}
	}

	var dest string
	if len(pos) > 1 {
		dest = pos[1]
	}
	if dest == "" {
		dest = repo
		if p.Root != "" {
			dest = filepath.Join(p.Root, repo)
		}
	}

	url := p.CloneURL(owner, repo)
	fmt.Printf("Cloning %s as %s (%s)\n", url, p.Name, p.Username)

	cmd := exec.Command("git", "clone", url, dest)
	cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	for key, val := range map[string]string{
		"user.name":  p.GitName,
		"user.email": p.GitEmail,
	} {
		c := exec.Command("git", "-C", dest, "config", key, val)
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			return fmt.Errorf("pinning %s in %s: %w", key, dest, err)
		}
	}
	if p.SigningKey != "" {
		for key, val := range map[string]string{
			"user.signingkey": p.SigningKey,
			"gpg.format":      p.SigningFormat,
			"commit.gpgsign":  "true",
		} {
			c := exec.Command("git", "-C", dest, "config", key, val)
			c.Stderr = os.Stderr
			if err := c.Run(); err != nil {
				return fmt.Errorf("pinning %s in %s: %w", key, dest, err)
			}
		}
	}

	fmt.Printf("\nCloned to %s, pinned to %s <%s>\n", dest, p.GitName, p.GitEmail)
	return nil
}

// splitRepo accepts owner/repo and tolerates a full GitHub URL, since that is
// what people actually have on the clipboard.
func splitRepo(s string) (owner, repo string, ok bool) {
	s = strings.TrimSuffix(s, ".git")
	if i := strings.Index(s, "github.com"); i >= 0 {
		s = s[i+len("github.com"):]
		s = strings.TrimLeft(s, ":/")
	}
	parts := strings.Split(strings.Trim(s, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}
