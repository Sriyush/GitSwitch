// Package apply writes the profile store out to the files git and ssh read.
//
// It exists as its own package because both the CLI and the management server
// mutate profiles, and both must produce byte-identical config afterwards. If
// each rendered its own, a profile edited in the browser and the same profile
// edited in a terminal would eventually disagree — which is the one failure this
// project cannot afford, since the disagreement would be silent and would show
// up as a commit under the wrong account.
package apply

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sriyush/gitswitch/internal/gitcfg"
	"github.com/sriyush/gitswitch/internal/profile"
	"github.com/sriyush/gitswitch/internal/sshcfg"
)

// Store rewrites every managed region from the current store. It is called after
// any mutation, so on-disk state always matches the store rather than
// accumulating the history of what was changed.
func Store(store *profile.Store) error {
	dir, err := profile.ConfigDir()
	if err != nil {
		return err
	}
	fragDir := filepath.Join(dir, "profiles")
	if err := os.MkdirAll(fragDir, 0o700); err != nil {
		return err
	}

	var all []gitcfg.Identity
	var active *gitcfg.Identity
	for _, p := range store.List() {
		id := gitcfg.Identity{
			Name:          p.Name,
			GitName:       p.GitName,
			GitEmail:      p.GitEmail,
			SigningKey:    p.SigningKey,
			SigningFormat: p.SigningFormat,
			Root:          p.Root,
			FragmentPath:  filepath.Join(fragDir, p.Name+".gitconfig"),
		}
		if err := os.WriteFile(id.FragmentPath, []byte(gitcfg.Fragment(id)), 0o600); err != nil {
			return err
		}
		all = append(all, id)
		if p.Name == store.Active {
			id := id
			active = &id
		}
	}

	if err := pruneFragments(fragDir, store); err != nil {
		return err
	}

	var hosts []sshcfg.Host
	for _, p := range store.List() {
		if p.SSHKey != "" {
			hosts = append(hosts, sshcfg.Host{Alias: p.DefaultHostAlias(), Key: p.SSHKey})
		}
	}
	if err := sshcfg.Apply(hosts); err != nil {
		return fmt.Errorf("updating ~/.ssh/config: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	return gitcfg.WriteBlock(filepath.Join(home, ".gitconfig"), gitcfg.GlobalBlock(active, all))
}

// pruneFragments deletes generated identity files whose profile no longer
// exists. Only files matching the exact name gitswitch generates are removed, so
// anything a user dropped into the directory by hand survives.
//
// Fragments are regenerated from the store on every mutation, which makes an
// orphan pure debris — but debris that still holds an old email, so it is worth
// clearing rather than explaining.
func pruneFragments(fragDir string, store *profile.Store) error {
	entries, err := os.ReadDir(fragDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".gitconfig") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".gitconfig")
		if _, live := store.Profiles[name]; live {
			continue
		}
		if err := os.Remove(filepath.Join(fragDir, e.Name())); err != nil {
			return fmt.Errorf("removing stale fragment %s: %w", e.Name(), err)
		}
	}
	return nil
}
