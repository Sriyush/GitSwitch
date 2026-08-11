package cli

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/sriyush/gitswitch/internal/apply"
	"github.com/sriyush/gitswitch/internal/profile"
)

// cmdEdit changes fields on an existing profile.
//
// Only flags actually present on the command line are applied. That distinction
// matters: it is what lets `--root ""` clear a value while an omitted --root
// leaves it alone. flag.Visit reports set flags only, so the zero value of an
// omitted flag is never mistaken for an intentional empty string.
func cmdEdit(args []string) error {
	fs := flag.NewFlagSet("edit", flag.ContinueOnError)
	var (
		username = fs.String("username", "", "GitHub login")
		gitName  = fs.String("name", "", "value for user.name")
		email    = fs.String("email", "", "value for user.email")
		sshKey   = fs.String("ssh-key", "", "path to the private SSH key")
		signKey  = fs.String("signing-key", "", "signing key; empty string disables signing")
		signFmt  = fs.String("signing-format", "", `"ssh" or "openpgp"`)
		root     = fs.String("root", "", "directory this profile owns; empty string removes the scope")
		orgs     = fs.String("orgs", "", "comma-separated GitHub owners; replaces the existing list")
	)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: gsw edit <profile> [flags]")
		fmt.Fprintln(os.Stderr, "\nOnly the flags you pass are changed. Pass an empty value to clear a field:")
		fmt.Fprintln(os.Stderr, "  gsw edit work --root ~/work")
		fmt.Fprintln(os.Stderr, `  gsw edit work --root ""`)
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

	store, err := profile.Load()
	if err != nil {
		return err
	}
	p, err := store.Get(pos[0])
	if err != nil {
		return err
	}

	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	if len(set) == 0 {
		return fmt.Errorf("no changes given; see `gsw edit -h`")
	}

	var changed []string
	update := func(name, old, new string) string {
		if !set[name] {
			return old
		}
		if old != new {
			changed = append(changed, fmt.Sprintf("  %s: %s -> %s", name, orNone(old), orNone(new)))
		}
		return new
	}

	p.Username = update("username", p.Username, *username)
	p.GitName = update("name", p.GitName, *gitName)
	p.GitEmail = update("email", p.GitEmail, *email)
	p.SSHKey = update("ssh-key", p.SSHKey, profile.ExpandPath(*sshKey))
	p.SigningKey = update("signing-key", p.SigningKey, profile.ExpandPath(*signKey))
	p.SigningFormat = update("signing-format", p.SigningFormat, *signFmt)
	p.Root = update("root", p.Root, profile.ExpandPath(*root))

	// Clearing the signing key must clear the format too, or Validate rejects a
	// format with no key behind it.
	if set["signing-key"] && p.SigningKey == "" {
		p.SigningFormat = ""
	}
	if p.SigningKey != "" && p.SigningFormat == "" {
		p.SigningFormat = "ssh"
	}

	if set["orgs"] {
		var list []string
		for _, o := range strings.Split(*orgs, ",") {
			if o = strings.TrimSpace(o); o != "" {
				list = append(list, o)
			}
		}
		if strings.Join(p.Orgs, ",") != strings.Join(list, ",") {
			changed = append(changed, fmt.Sprintf("  orgs: %s -> %s",
				orNone(strings.Join(p.Orgs, ",")), orNone(strings.Join(list, ","))))
		}
		p.Orgs = list
	}

	if err := p.Validate(); err != nil {
		return err
	}
	if err := store.CheckRoot(p.Name, p.Root); err != nil {
		return err
	}
	if len(changed) == 0 {
		fmt.Printf("No changes to %q.\n", p.Name)
		return nil
	}
	if err := store.Save(); err != nil {
		return err
	}
	if err := apply.Store(store); err != nil {
		return err
	}

	fmt.Printf("Updated %q:\n%s\n", p.Name, strings.Join(changed, "\n"))
	if set["root"] && p.Root != "" {
		fmt.Printf("\nRepos under %s now always use this identity.\n", p.Root)
	}
	return nil
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}
