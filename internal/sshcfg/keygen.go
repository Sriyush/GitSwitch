package sshcfg

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DefaultKeyPath is where a generated key for a profile lives.
//
// Each GitHub account needs its own key: GitHub rejects a public key that is
// already registered to another account, so sharing one key across profiles is
// not merely bad practice, it is impossible.
func DefaultKeyPath(profileName string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ssh", "id_ed25519_"+profileName), nil
}

// GenerateKey creates an ed25519 keypair at path, or returns the existing one
// untouched. It shells out to ssh-keygen rather than encoding the OpenSSH
// private key format by hand, which keeps the project dependency-free.
func GenerateKey(path, comment string) (created bool, err error) {
	if _, err := os.Stat(path); err == nil {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return false, err
	}
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		return false, fmt.Errorf("ssh-keygen not found; install openssh and retry, or pass --ssh-key")
	}

	cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-f", path, "-N", "", "-C", comment, "-q")
	if out, err := cmd.CombinedOutput(); err != nil {
		return false, fmt.Errorf("ssh-keygen: %s", strings.TrimSpace(string(out)))
	}
	return true, nil
}

// PublicKey reads the .pub half of a private key path.
func PublicKey(privatePath string) (string, error) {
	data, err := os.ReadFile(privatePath + ".pub")
	if err != nil {
		return "", fmt.Errorf("reading public key: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// Clipboard copies text using whichever helper this desktop provides. Failure is
// not an error worth surfacing: the key is printed to the terminal regardless,
// so the user can always select it by hand.
func Clipboard(text string) bool {
	candidates := [][]string{
		{"wl-copy"},
		{"xclip", "-selection", "clipboard"},
		{"xsel", "--clipboard", "--input"},
		{"pbcopy"},
	}
	for _, argv := range candidates {
		if _, err := exec.LookPath(argv[0]); err != nil {
			continue
		}
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err == nil {
			return true
		}
	}
	return false
}
