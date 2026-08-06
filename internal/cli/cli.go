// Package cli implements the gsw command line.
//
// Dispatch is deliberately dependency-free so the scaffold builds offline. The
// command table maps cleanly onto cobra later if richer help and completion are
// wanted; nothing outside this file assumes either.
package cli

import (
	"fmt"
	"os"
)

type command struct {
	name    string
	summary string
	run     func(args []string) error
}

func commands() []command {
	return []command{
		{"add", "Register a new GitHub account profile", cmdAdd},
		{"list", "List all profiles", cmdList},
		{"edit", "Change fields on an existing profile", cmdEdit},
		{"key", "Show a profile's public key and how to add it to GitHub", cmdKey},
		{"switch", "Make a profile the active identity", cmdSwitch},
		{"status", "Show the active profile and the identity in this directory", cmdStatus},
		{"remove", "Delete a profile", cmdRemove},
		{"clone", "Clone a repo using the profile that owns it", cmdClone},
		{"doctor", "Diagnose SSH, token, signing, and config problems", cmdDoctor},
		{"ui", "Start the local management server and open the web UI", cmdUI},
		{"restore", "Remove all gitswitch-managed config blocks", cmdRestore},
	}
}

// aliases keeps the common shorthands out of the help table.
var aliases = map[string]string{
	"ls": "list",
	"rm": "remove",
	"sw": "switch",
	"st": "status",
}

// splitPositional separates leading positional arguments from flag arguments.
//
// Go's flag package stops parsing at the first non-flag argument, so
// `gsw add work --username x` would otherwise treat --username and its value as
// positionals. Pulling the leading operands off first lets flags appear on
// either side of them, which is what users expect.
func splitPositional(args []string) (pos, flags []string) {
	i := 0
	for i < len(args) && (args[i] == "" || args[i][0] != '-') {
		i++
	}
	return args[:i], args[i:]
}

// Run dispatches argv and returns a process exit code.
func Run(args []string, version string) int {
	if len(args) == 0 {
		usage(version)
		return 0
	}

	name := args[0]
	rest := args[1:]

	switch name {
	case "-h", "--help", "help":
		usage(version)
		return 0
	case "-v", "--version", "version":
		fmt.Printf("gsw %s\n", version)
		return 0
	}

	if target, ok := aliases[name]; ok {
		name = target
	}

	for _, c := range commands() {
		if c.name == name {
			if err := c.run(rest); err != nil {
				fmt.Fprintf(os.Stderr, "gsw: %v\n", err)
				return 1
			}
			return 0
		}
	}

	// Bare `gsw work` is shorthand for `gsw switch work`. Switching is by far the
	// most frequent operation, so it gets the shortest possible form.
	if len(rest) == 0 && name != "" && name[0] != '-' {
		if err := cmdSwitch([]string{name}); err != nil {
			fmt.Fprintf(os.Stderr, "gsw: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Fprintf(os.Stderr, "gsw: unknown command %q\nRun `gsw help` for usage.\n", args[0])
	return 1
}

func usage(version string) {
	fmt.Printf("gsw %s - switch GitHub identities across git, SSH, tokens, and signing\n\n", version)
	fmt.Println("USAGE")
	fmt.Println("  gsw <command> [arguments]")
	fmt.Println("  gsw <profile>              shorthand for `gsw switch <profile>`")
	fmt.Println()
	fmt.Println("COMMANDS")
	for _, c := range commands() {
		fmt.Printf("  %-9s %s\n", c.name, c.summary)
	}
	fmt.Println()
	fmt.Println("Run `gsw <command> -h` for command-specific flags.")
}
