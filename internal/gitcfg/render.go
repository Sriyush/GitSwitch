package gitcfg

import (
	"fmt"
	"strings"
)

// Identity is the subset of a profile that git itself cares about. Keeping this
// package free of any dependency on the profile package makes the config
// rendering independently testable.
type Identity struct {
	Name          string // profile handle, used for the fragment filename
	GitName       string
	GitEmail      string
	SigningKey    string
	SigningFormat string // "ssh" or "openpgp"
	Root          string // directory this identity owns, for includeIf
	FragmentPath  string // where this identity's .gitconfig fragment lives
}

// Fragment renders a standalone identity file, included by path from
// ~/.gitconfig. Directory-scoped switching works by including one of these under
// an includeIf rule, so git applies the right identity with no process running.
func Fragment(id Identity) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# gitswitch profile: %s\n", id.Name)
	fmt.Fprintf(&b, "# Generated file. Edit via `gsw edit %s`.\n\n", id.Name)
	b.WriteString("[user]\n")
	fmt.Fprintf(&b, "\tname = %s\n", id.GitName)
	fmt.Fprintf(&b, "\temail = %s\n", id.GitEmail)

	if id.SigningKey != "" {
		fmt.Fprintf(&b, "\tsigningkey = %s\n", id.SigningKey)
		b.WriteString("\n[gpg]\n")
		fmt.Fprintf(&b, "\tformat = %s\n", id.SigningFormat)
		b.WriteString("\n[commit]\n\tgpgsign = true\n")
	}
	return b.String()
}

// GlobalBlock renders the managed region of ~/.gitconfig: the active identity,
// plus an includeIf rule per profile that owns a directory.
//
// Order matters. Git applies includes in file order, so the directory rules come
// after the active identity and therefore win inside their own trees. That is
// what makes "I forgot to switch" a non-event.
func GlobalBlock(active *Identity, all []Identity) string {
	var b strings.Builder

	if active != nil {
		fmt.Fprintf(&b, "# Active profile: %s\n", active.Name)
		b.WriteString("[include]\n")
		fmt.Fprintf(&b, "\tpath = %s\n", active.FragmentPath)
	} else {
		b.WriteString("# No active profile. Run `gsw switch <name>`.\n")
	}

	var scoped []Identity
	for _, id := range all {
		if id.Root != "" {
			scoped = append(scoped, id)
		}
	}
	if len(scoped) > 0 {
		b.WriteString("\n# Directory-scoped identities. These override the active\n")
		b.WriteString("# profile inside their own trees.\n")
		for _, id := range scoped {
			root := strings.TrimSuffix(id.Root, "/") + "/"
			fmt.Fprintf(&b, "\n[includeIf \"gitdir:%s\"]\n", root)
			fmt.Fprintf(&b, "\tpath = %s\n", id.FragmentPath)
		}
	}
	return b.String()
}
