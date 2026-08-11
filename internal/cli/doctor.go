package cli

import (
	"flag"
	"fmt"

	"github.com/sriyush/gitswitch/internal/checkup"
	"github.com/sriyush/gitswitch/internal/profile"
)

// cmdDoctor renders the diagnostics from internal/checkup.
//
// The checks themselves live in that package because the web UI serves the same
// results; this function only decides how they look in a terminal.
func cmdDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	offline := fs.Bool("offline", false, "skip checks that contact GitHub")
	if err := fs.Parse(args); err != nil {
		return err
	}

	store, err := profile.Load()
	if err != nil {
		return err
	}
	if len(store.List()) == 0 {
		fmt.Println("No profiles configured. Run `gsw add <name> --username <login> --email <email>`.")
		return nil
	}

	res, err := checkup.Run(store, *offline)
	if err != nil {
		return err
	}

	for _, rep := range res.Profiles {
		fmt.Printf("\n%s (%s)\n", rep.Profile, rep.Username)
		for _, c := range rep.Checks {
			printCheck("  ", c)
		}
	}

	fmt.Println()
	for _, c := range res.Global {
		printCheck("", c)
	}

	if res.Failed > 0 {
		return fmt.Errorf("%d check(s) failed", res.Failed)
	}
	fmt.Println("\nAll local checks passed.")
	return nil
}

// printCheck renders one check. Skipped checks use a distinct marker so they
// cannot be misread as passing.
func printCheck(indent string, c checkup.Check) {
	switch c.Status {
	case checkup.StatusSkip:
		fmt.Printf("%s~ %-24s %s\n", indent, c.Label, c.Detail)
	case checkup.StatusPass:
		fmt.Printf("%s+ %-24s %s\n", indent, c.Label, c.Detail)
	default:
		fmt.Printf("%sx %-24s %s\n", indent, c.Label, c.Detail)
		if c.Remedy != "" {
			fmt.Printf("%s  -> %s\n", indent, c.Remedy)
		}
	}
}
