package cli

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sriyush/gitswitch/internal/apply"
	"github.com/sriyush/gitswitch/internal/gitcfg"
	"github.com/sriyush/gitswitch/internal/hook"
	"github.com/sriyush/gitswitch/internal/profile"
	"github.com/sriyush/gitswitch/internal/sshcfg"
)

func cmdAdd(args []string) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	var (
		username = fs.String("username", "", "GitHub login (required)")
		gitName  = fs.String("name", "", "value for user.name (defaults to --username)")
		email    = fs.String("email", "", "value for user.email (required)")
		sshKey   = fs.String("ssh-key", "", "use an existing private key instead of generating one")
		noKey    = fs.Bool("no-key", false, "skip SSH key setup entirely")
		signKey  = fs.String("signing-key", "", "SSH public key or GPG key id for commit signing")
		signFmt  = fs.String("signing-format", "ssh", `"ssh" or "openpgp"`)
		root     = fs.String("root", "", "directory this profile owns; repos under it always use this identity")
		orgs     = fs.String("orgs", "", "comma-separated GitHub owners routed to this profile")
	)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: gsw add <profile> --username <login> --email <email> [flags]")
		fs.PrintDefaults()
	}
	pos, flags := splitPositional(args)
	if err := fs.Parse(flags); err != nil {
		return err
	}
	pos = append(pos, fs.Args()...)
	if len(pos) != 1 {
		fs.Usage()
		return fmt.Errorf("expected exactly one profile name")
	}

	name := pos[0]
	if *username == "" {
		return fmt.Errorf("--username is required")
	}
	if *email == "" {
		return fmt.Errorf("--email is required")
	}
	if *gitName == "" {
		*gitName = *username
	}
	if *signKey == "" {
		*signFmt = ""
	}

	// Every account needs its own key, since GitHub refuses a public key that is
	// already registered elsewhere. Generating one by default removes the most
	// tedious part of adding an account.
	keyPath := profile.ExpandPath(*sshKey)
	generated := false
	if keyPath == "" && !*noKey {
		var err error
		if keyPath, err = sshcfg.DefaultKeyPath(name); err != nil {
			return err
		}
		if generated, err = sshcfg.GenerateKey(keyPath, "gitswitch-"+*username); err != nil {
			return err
		}
	}

	p := &profile.Profile{
		Name:          name,
		Username:      *username,
		GitName:       *gitName,
		GitEmail:      *email,
		SSHKey:        keyPath,
		SigningKey:    profile.ExpandPath(*signKey),
		SigningFormat: *signFmt,
		Root:          profile.ExpandPath(*root),
		TokenRef:      "keyring://gitswitch/" + name,
	}
	p.HostAlias = p.DefaultHostAlias()
	if *orgs != "" {
		for _, o := range strings.Split(*orgs, ",") {
			if o = strings.TrimSpace(o); o != "" {
				p.Orgs = append(p.Orgs, o)
			}
		}
	}

	store, err := profile.Load()
	if err != nil {
		return err
	}
	if err := store.CheckRoot(p.Name, p.Root); err != nil {
		return err
	}
	if err := store.Add(p); err != nil {
		return err
	}
	if err := store.Save(); err != nil {
		return err
	}
	if err := apply.Store(store); err != nil {
		return err
	}

	fmt.Printf("Added profile %q (%s <%s>)\n", p.Name, p.GitName, p.GitEmail)
	if p.Root != "" {
		fmt.Printf("  Repos under %s will always use this identity.\n", p.Root)
	}
	if generated {
		fmt.Printf("  Generated a new SSH key at %s\n", p.SSHKey)
	} else if p.SSHKey != "" {
		fmt.Printf("  Using SSH key %s\n", p.SSHKey)
	}
	if p.SSHKey != "" {
		fmt.Println()
		return showKey(p)
	}
	return nil
}

// showKey prints the public key and what to do with it. GitHub will not accept
// a key already registered to another account, so this step cannot be shared
// between profiles and has to be done once per account.
func showKey(p *profile.Profile) error {
	if p.SSHKey == "" {
		return fmt.Errorf("profile %q has no SSH key; set one with `gsw edit %s --ssh-key <path>`", p.Name, p.Name)
	}
	pub, err := sshcfg.PublicKey(p.SSHKey)
	if err != nil {
		return err
	}

	fmt.Println("Add this public key to GitHub to finish setup:")
	fmt.Println()
	fmt.Printf("  %s\n", pub)
	fmt.Println()
	if sshcfg.Clipboard(pub) {
		fmt.Println("  (copied to clipboard)")
	}
	fmt.Printf("  1. Sign in to GitHub as %s\n", p.Username)
	fmt.Println("  2. Open https://github.com/settings/ssh/new")
	fmt.Println("  3. Paste the key, leave the type as \"Authentication Key\", and save")
	fmt.Printf("\nThen verify with: gsw doctor\n")
	return nil
}

func cmdKey(args []string) error {
	store, err := profile.Load()
	if err != nil {
		return err
	}

	var p *profile.Profile
	switch len(args) {
	case 0:
		var ok bool
		if p, ok = store.ActiveProfile(); !ok {
			return fmt.Errorf("no active profile; usage: gsw key <profile>")
		}
	case 1:
		if p, err = store.Get(args[0]); err != nil {
			return err
		}
	default:
		return fmt.Errorf("usage: gsw key [profile]")
	}
	return showKey(p)
}

func cmdList(args []string) error {
	store, err := profile.Load()
	if err != nil {
		return err
	}
	list := store.List()
	if len(list) == 0 {
		fmt.Println("No profiles yet. Add one with:")
		fmt.Println("  gsw add personal --username <login> --email <email>")
		return nil
	}
	for _, p := range list {
		marker := " "
		if p.Name == store.Active {
			marker = "*"
		}
		fmt.Printf("%s %-12s %-20s %s\n", marker, p.Name, p.Username, p.GitEmail)
		if p.Root != "" {
			fmt.Printf("  %-12s scope: %s\n", "", p.Root)
		}
	}
	return nil
}

func cmdSwitch(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: gsw switch <profile>")
	}
	store, err := profile.Load()
	if err != nil {
		return err
	}
	p, err := store.Get(args[0])
	if err != nil {
		return err
	}
	store.Active = p.Name
	if err := store.Save(); err != nil {
		return err
	}
	if err := apply.Store(store); err != nil {
		return err
	}
	fmt.Printf("Switched to %s (%s <%s>)\n", p.Name, p.Username, p.GitEmail)
	return nil
}

func cmdStatus(args []string) error {
	store, err := profile.Load()
	if err != nil {
		return err
	}
	p, ok := store.ActiveProfile()
	if !ok {
		fmt.Println("No active profile. Run `gsw switch <name>`.")
		return nil
	}
	fmt.Printf("Active profile : %s (%s)\n", p.Name, p.Username)
	fmt.Printf("Commit identity: %s <%s>\n", p.GitName, p.GitEmail)

	// The effective identity is what git reports here, which can differ from the
	// active profile when an includeIf rule applies. That difference is the whole
	// point of directory scoping, so surface it rather than hiding it.
	if email, err := gitConfigValue("user.email"); err == nil && email != "" {
		fmt.Printf("This directory : %s\n", email)
		if email != p.GitEmail {
			owner := "a directory-scoped rule"
			for _, other := range store.List() {
				if other.GitEmail == email {
					owner = fmt.Sprintf("profile %q", other.Name)
					break
				}
			}
			fmt.Printf("                 (overridden by %s)\n", owner)
		}
	}
	return nil
}

func cmdRemove(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: gsw remove <profile>")
	}
	store, err := profile.Load()
	if err != nil {
		return err
	}
	removed, err := store.Get(args[0])
	if err != nil {
		return err
	}
	if err := store.Remove(args[0]); err != nil {
		return err
	}
	if err := store.Save(); err != nil {
		return err
	}
	if err := apply.Store(store); err != nil {
		return err
	}
	fmt.Printf("Removed profile %q\n", args[0])

	// Key material is deliberately never deleted here: an SSH key may be
	// registered on GitHub, referenced by another tool, or simply irreplaceable,
	// and removing one by surprise is far worse than leaving a stale file. Say so
	// rather than leaving the user to discover it.
	if removed != nil && removed.SSHKey != "" {
		fmt.Printf("  The SSH key at %s was kept. Delete it yourself if you no longer want it.\n", removed.SSHKey)
	}
	return nil
}

// cmdRestore returns the machine to its pre-gitswitch state.
//
// It clears every managed region, not just the gitconfig block. Clearing one of
// three left the machine in a state it had never been in: no identity
// configured, but host aliases still resolving and a guard still running.
func cmdRestore(args []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// Each region is reported only if it was actually there. A restore that
	// announces removals it did not perform teaches the user to distrust the
	// output, which matters more here than anywhere else: this is the command
	// people run when they want to be sure nothing of ours is left behind.
	gitconfigPath := filepath.Join(home, ".gitconfig")
	_, had, err := gitcfg.Block(gitconfigPath)
	if err != nil {
		return err
	}
	if err := gitcfg.RemoveBlock(gitconfigPath); err != nil {
		return err
	}
	if had {
		fmt.Printf("Removed the gitswitch block from %s\n", gitconfigPath)
		fmt.Printf("  A backup of the previous contents is at %s.gsw-backup\n", gitconfigPath)
	} else {
		fmt.Printf("No gitswitch block in %s\n", gitconfigPath)
	}

	sshPath, err := sshcfg.Path()
	if err != nil {
		return err
	}
	_, hadSSH, err := gitcfg.Block(sshPath)
	if err != nil {
		return err
	}
	if err := sshcfg.Apply(nil); err != nil {
		return fmt.Errorf("clearing %s: %w", sshPath, err)
	}
	if hadSSH {
		fmt.Printf("Removed the gitswitch block from %s\n", sshPath)
	} else {
		fmt.Printf("No gitswitch block in %s\n", sshPath)
	}

	switch installed, _, err := hook.Installed(); {
	case err != nil:
		return err
	case installed:
		if err := hook.Uninstall(); err != nil {
			return err
		}
		fmt.Println("Removed the pre-push guard and cleared core.hooksPath")
	}

	fmt.Println("\nProfiles themselves are untouched. `gsw switch <name>` writes all of this back.")
	return nil
}

func gitConfigValue(key string) (string, error) {
	out, err := exec.Command("git", "config", "--get", key).Output()
	return strings.TrimSpace(string(out)), err
}
