package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/sriyush/gitswitch/internal/hook"
	"github.com/sriyush/gitswitch/internal/profile"
)

func cmdHook(args []string) error {
	action := "status"
	if len(args) > 0 {
		action = args[0]
	}

	switch action {
	case "install":
		path, err := hook.Install()
		if err != nil {
			return err
		}
		fmt.Printf("Installed the pre-push guard at %s\n", path)
		fmt.Println("Pushes to a repo owned by a different profile will now be blocked.")
		fmt.Println("\nBypass a single push with: GSW_SKIP_GUARD=1 git push")
		return nil

	case "uninstall":
		if err := hook.Uninstall(); err != nil {
			return err
		}
		fmt.Println("Removed the pre-push guard.")
		return nil

	case "status":
		installed, path, err := hook.Installed()
		if err != nil {
			return err
		}
		if installed {
			fmt.Printf("Pre-push guard is active (core.hooksPath = %s)\n", path)
		} else if path != "" {
			fmt.Printf("Not installed. core.hooksPath points elsewhere: %s\n", path)
		} else {
			fmt.Println("Not installed. Run `gsw hook install`.")
		}
		return nil

	default:
		return fmt.Errorf("usage: gsw hook [install|uninstall|status]")
	}
}

// cmdCheckPush is invoked by the pre-push hook, not by hand.
//
// It blocks only when it is confident: a profile must explicitly claim the
// repository owner, and the identity in play must belong to a different
// profile. Anything ambiguous is allowed through, because a guard that cries
// wolf gets uninstalled.
func cmdCheckPush(args []string) error {
	if len(args) < 2 {
		return nil // Not enough context to judge; never block on our own confusion.
	}
	owner, repo, ok := splitRepo(args[1])
	if !ok {
		return nil
	}

	store, err := profile.Load()
	if err != nil {
		return nil
	}
	expected, found := store.ResolveOwner(owner)
	if !found {
		return nil // No profile claims this owner, so there is nothing to compare.
	}

	email, err := gitConfigValue("user.email")
	if err != nil || email == "" {
		return nil
	}
	if strings.EqualFold(email, expected.GitEmail) {
		return nil
	}

	current := "an identity with no matching profile"
	var currentName string
	for _, p := range store.List() {
		if strings.EqualFold(p.GitEmail, email) {
			current = fmt.Sprintf("%s (%s)", p.Name, p.Username)
			currentName = p.Name
			break
		}
	}

	fmt.Fprintf(os.Stderr, "\ngitswitch: blocked push to %s/%s\n\n", owner, repo)
	fmt.Fprintf(os.Stderr, "  This repo belongs to %s (%s)\n", expected.Name, expected.Username)
	fmt.Fprintf(os.Stderr, "  You are committing as %s <%s>\n\n", current, email)
	fmt.Fprintf(os.Stderr, "  Fix:    gsw switch %s\n", expected.Name)
	if currentName != "" {
		fmt.Fprintf(os.Stderr, "  Or:     gsw edit %s --orgs %s   (if %s really owns it)\n",
			currentName, owner, currentName)
	}
	fmt.Fprintf(os.Stderr, "  Bypass: GSW_SKIP_GUARD=1 git push\n\n")

	return fmt.Errorf("identity mismatch")
}
