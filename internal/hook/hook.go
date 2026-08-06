// Package hook installs the pre-push guard.
//
// Switching identities is easy; noticing that you forgot is not. The guard turns
// a silent wrong-account push into a blocked one with a one-line fix.
package hook

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// script is the installed pre-push hook.
//
// It chains to the repository's own .git/hooks/pre-push when one exists. Setting
// core.hooksPath globally otherwise shadows every repo's local hooks, which
// would silently disable things like husky or lint gates — an unacceptable side
// effect for a tool that is supposed to prevent surprises.
const script = `#!/bin/sh
# Installed by gitswitch. Remove with: gsw hook uninstall
if [ -z "$GSW_SKIP_GUARD" ]; then
	if command -v gsw >/dev/null 2>&1; then
		gsw check-push "$@" </dev/null || exit 1
	fi
fi

# Chain to the repository's own pre-push hook, if it has one.
own=$(git rev-parse --git-path hooks/pre-push 2>/dev/null)
if [ -n "$own" ] && [ -x "$own" ]; then
	exec "$own" "$@"
fi
exit 0
`

// Dir returns the directory git is pointed at via core.hooksPath.
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "gitswitch", "hooks"), nil
}

// Install writes the hook and points core.hooksPath at it.
func Install() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "pre-push")
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		return "", err
	}

	if existing, err := gitConfig("--global", "core.hooksPath"); err == nil {
		if existing != "" && existing != dir {
			return "", fmt.Errorf("core.hooksPath is already set to %s; "+
				"gitswitch will not overwrite it. Point it at %s manually, or copy the "+
				"guard into that directory", existing, dir)
		}
	}
	if err := setGitConfig("--global", "core.hooksPath", dir); err != nil {
		return "", err
	}
	return path, nil
}

// Uninstall removes the hook and clears core.hooksPath if we set it.
func Uninstall() error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	current, _ := gitConfig("--global", "core.hooksPath")
	if current == dir {
		cmd := exec.Command("git", "config", "--global", "--unset", "core.hooksPath")
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("clearing core.hooksPath: %s", strings.TrimSpace(string(out)))
		}
	}
	return os.Remove(filepath.Join(dir, "pre-push"))
}

// Installed reports whether the guard is active.
func Installed() (bool, string, error) {
	dir, err := Dir()
	if err != nil {
		return false, "", err
	}
	current, _ := gitConfig("--global", "core.hooksPath")
	if current != dir {
		return false, current, nil
	}
	if _, err := os.Stat(filepath.Join(dir, "pre-push")); err != nil {
		return false, current, nil
	}
	return true, current, nil
}

func gitConfig(args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"config", "--get"}, args...)...).Output()
	return strings.TrimSpace(string(out)), err
}

func setGitConfig(args ...string) error {
	cmd := exec.Command("git", append([]string{"config"}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git config: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
