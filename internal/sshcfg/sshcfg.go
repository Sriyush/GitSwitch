package sshcfg

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sriyush/gitswitch/internal/gitcfg"
)

// Host is one GitHub account's SSH alias.
type Host struct {
	Alias string // e.g. "github.com-work"
	Key   string // path to the private key
}

// Path returns ~/.ssh/config.
func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ssh", "config"), nil
}

// Render builds the managed block.
//
// IdentitiesOnly yes is the load-bearing line. Without it ssh offers every key
// the agent holds and GitHub authenticates as whichever one matches first, which
// is the single most common cause of "I pushed as the wrong account" — and it
// fails silently, because the push succeeds.
func Render(hosts []Host) string {
	var b strings.Builder
	for i, h := range hosts {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "Host %s\n", h.Alias)
		b.WriteString("\tHostName github.com\n")
		b.WriteString("\tUser git\n")
		if h.Key != "" {
			fmt.Fprintf(&b, "\tIdentityFile %s\n", h.Key)
		}
		b.WriteString("\tIdentitiesOnly yes\n")
		b.WriteString("\tAddKeysToAgent yes\n")
	}
	return b.String()
}

// Apply writes the managed block to ~/.ssh/config, preserving every entry the
// user wrote by hand.
func Apply(hosts []Host) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if len(hosts) == 0 {
		return gitcfg.RemoveBlock(path)
	}
	if err := gitcfg.WriteBlock(path, Render(hosts)); err != nil {
		return err
	}
	// ssh refuses to use a config group- or world-readable.
	return os.Chmod(path, 0o600)
}

// greeting matches GitHub's SSH banner: "Hi <login>! You've successfully..."
var greeting = regexp.MustCompile(`Hi ([A-Za-z0-9-]+)!`)

// Verify opens an SSH session to the alias and reports which GitHub account
// answered. GitHub always exits non-zero here since it grants no shell, so the
// banner is parsed rather than the exit status.
func Verify(alias string) (login string, err error) {
	cmd := exec.Command("ssh", "-o", "StrictHostKeyChecking=accept-new", "-o", "BatchMode=yes", "-T", "git@"+alias)
	out, _ := cmd.CombinedOutput()

	if m := greeting.FindSubmatch(out); m != nil {
		return string(m[1]), nil
	}
	msg := strings.TrimSpace(string(out))
	if msg == "" {
		msg = "no response from " + alias
	}
	return "", fmt.Errorf("%s", firstLine(msg))
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
