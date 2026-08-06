package cli

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sriyush/gitswitch/internal/gitcfg"
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
	keyPath := expand(*sshKey)
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
		SigningKey:    expand(*signKey),
		SigningFormat: *signFmt,
		Root:          expand(*root),
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
	if err := store.Add(p); err != nil {
		return err
	}
	if err := store.Save(); err != nil {
		return err
	}
	if err := applyStore(store); err != nil {
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
	if err := applyStore(store); err != nil {
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
	if err := store.Remove(args[0]); err != nil {
		return err
	}
	if err := store.Save(); err != nil {
		return err
	}
	if err := applyStore(store); err != nil {
		return err
	}
	fmt.Printf("Removed profile %q\n", args[0])
	return nil
}

func cmdRestore(args []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	path := filepath.Join(home, ".gitconfig")
	if err := gitcfg.RemoveBlock(path); err != nil {
		return err
	}
	fmt.Printf("Removed the gitswitch block from %s\n", path)
	fmt.Printf("A backup of the previous contents is at %s.gsw-backup\n", path)
	return nil
}

// apply rewrites every managed config region from the current store. It is
// called after any mutation so disk state always matches the store.
func applyStore(store *profile.Store) error {
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

func gitConfigValue(key string) (string, error) {
	out, err := exec.Command("git", "config", "--get", key).Output()
	return strings.TrimSpace(string(out)), err
}

// expand resolves a leading ~ so flag values can be written the way users type
// paths in a shell.
func expand(path string) string {
	if path == "" {
		return ""
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~"))
		}
	}
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}
