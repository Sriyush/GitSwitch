// Package gitcfg edits config files in place without ever clobbering content
// the user wrote themselves.
//
// Every gitswitch-owned region is fenced by begin/end markers. Writes replace
// only what is between the markers; anything outside is preserved byte for byte.
// The same mechanism is reused for ~/.ssh/config, which has different syntax but
// the same "# comment" convention.
package gitcfg

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	MarkerBegin = "# >>> gitswitch managed >>>"
	MarkerEnd   = "# <<< gitswitch managed <<<"

	warning = "# Do not edit inside this block. Changes are overwritten by `gsw`."
)

// Block returns the managed region of the file, and whether one was present.
func Block(path string) (string, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	body, _, ok := split(string(data))
	return body, ok, nil
}

// WriteBlock replaces the managed region with content, creating the file and the
// block if either is missing. A timestamped backup is taken before the first
// write of a session so a bad apply is always recoverable.
func WriteBlock(path, content string) error {
	orig := ""
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		orig = string(data)
	case !os.IsNotExist(err):
		return err
	}

	if orig != "" {
		if err := backup(path, []byte(orig)); err != nil {
			return fmt.Errorf("backing up %s: %w", path, err)
		}
	}

	block := MarkerBegin + "\n" + warning + "\n" + strings.TrimRight(content, "\n") + "\n" + MarkerEnd

	var out string
	if _, after, ok := split(orig); ok {
		out = beforeOf(orig) + block + after
	} else {
		out = orig
		if out != "" && !strings.HasSuffix(out, "\n") {
			out += "\n"
		}
		if out != "" {
			out += "\n"
		}
		out += block + "\n"
	}

	return atomicWrite(path, []byte(out))
}

// RemoveBlock strips the managed region, leaving user content untouched. This is
// what `gsw restore` runs.
func RemoveBlock(path string) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	orig := string(data)
	if _, after, ok := split(orig); ok {
		if err := backup(path, data); err != nil {
			return err
		}
		out := strings.TrimRight(beforeOf(orig), "\n")
		rest := strings.TrimLeft(after, "\n")
		if out != "" && rest != "" {
			out += "\n\n"
		} else if out != "" {
			out += "\n"
		}
		return atomicWrite(path, []byte(out+rest))
	}
	return nil
}

// split cuts the file into the text before the block, the block body, and the
// text after. ok is false when no complete marker pair is present.
func split(s string) (body, after string, ok bool) {
	i := strings.Index(s, MarkerBegin)
	if i < 0 {
		return "", "", false
	}
	j := strings.Index(s[i:], MarkerEnd)
	if j < 0 {
		return "", "", false
	}
	j += i

	inner := s[i+len(MarkerBegin) : j]
	inner = strings.TrimPrefix(inner, "\n")
	inner = strings.TrimPrefix(inner, warning+"\n")

	return inner, s[j+len(MarkerEnd):], true
}

func beforeOf(s string) string {
	if i := strings.Index(s, MarkerBegin); i >= 0 {
		return s[:i]
	}
	return s
}

func backup(path string, data []byte) error {
	dst := path + ".gsw-backup"
	if existing, err := os.ReadFile(dst); err == nil && bytes.Equal(existing, data) {
		return nil
	}
	return os.WriteFile(dst, data, 0o600)
}

// atomicWrite writes via a temp file in the same directory then renames, so a
// crash mid-write cannot leave a half-written ~/.gitconfig behind.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".gsw-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)

	mode := os.FileMode(0o600)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
