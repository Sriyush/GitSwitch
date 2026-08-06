package gitcfg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const userContent = `[user]
	name = Sriyush
	email = asish.ayush.sri@gmail.com

[alias]
	st = status
`

// The whole design rests on never destroying config the user wrote by hand, so
// that property gets tested directly.
func TestWriteBlockPreservesUserContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".gitconfig")
	if err := os.WriteFile(path, []byte(userContent), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := WriteBlock(path, "[include]\n\tpath = /tmp/work.gitconfig\n"); err != nil {
		t.Fatal(err)
	}

	got := read(t, path)
	if !strings.Contains(got, "st = status") {
		t.Errorf("user alias was lost:\n%s", got)
	}
	if !strings.Contains(got, "path = /tmp/work.gitconfig") {
		t.Errorf("managed content missing:\n%s", got)
	}
	if strings.Count(got, MarkerBegin) != 1 {
		t.Errorf("expected exactly one begin marker, got %d", strings.Count(got, MarkerBegin))
	}
}

// Repeated switching must not append a new block every time.
func TestWriteBlockIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".gitconfig")
	if err := os.WriteFile(path, []byte(userContent), 0o600); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 5; i++ {
		if err := WriteBlock(path, "[include]\n\tpath = /tmp/personal.gitconfig\n"); err != nil {
			t.Fatal(err)
		}
	}

	got := read(t, path)
	if n := strings.Count(got, MarkerBegin); n != 1 {
		t.Errorf("expected 1 block after 5 writes, got %d:\n%s", n, got)
	}
	if n := strings.Count(got, "personal.gitconfig"); n != 1 {
		t.Errorf("expected content once, got %d times", n)
	}
}

func TestWriteBlockReplacesPreviousContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".gitconfig")
	if err := WriteBlock(path, "[include]\n\tpath = /tmp/one.gitconfig\n"); err != nil {
		t.Fatal(err)
	}
	if err := WriteBlock(path, "[include]\n\tpath = /tmp/two.gitconfig\n"); err != nil {
		t.Fatal(err)
	}

	got := read(t, path)
	if strings.Contains(got, "one.gitconfig") {
		t.Errorf("stale content survived:\n%s", got)
	}
	if !strings.Contains(got, "two.gitconfig") {
		t.Errorf("new content missing:\n%s", got)
	}
}

func TestRemoveBlockRestoresUserContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".gitconfig")
	if err := os.WriteFile(path, []byte(userContent), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteBlock(path, "[include]\n\tpath = /tmp/work.gitconfig\n"); err != nil {
		t.Fatal(err)
	}
	if err := RemoveBlock(path); err != nil {
		t.Fatal(err)
	}

	got := read(t, path)
	if strings.Contains(got, MarkerBegin) || strings.Contains(got, "work.gitconfig") {
		t.Errorf("block survived removal:\n%s", got)
	}
	if !strings.Contains(got, "st = status") || !strings.Contains(got, "name = Sriyush") {
		t.Errorf("user content damaged:\n%s", got)
	}
}

func TestWriteBlockCreatesMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", ".gitconfig")
	if err := WriteBlock(path, "[include]\n\tpath = /tmp/x.gitconfig\n"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(t, path), "x.gitconfig") {
		t.Error("file was not created with the managed block")
	}
}

func TestBlockRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".gitconfig")
	content := "[include]\n\tpath = /tmp/work.gitconfig"
	if err := WriteBlock(path, content); err != nil {
		t.Fatal(err)
	}
	body, ok, err := Block(path)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("block not detected")
	}
	if strings.TrimSpace(body) != content {
		t.Errorf("round trip mismatch:\ngot  %q\nwant %q", strings.TrimSpace(body), content)
	}
}

func TestGlobalBlockOrdersScopedRulesLast(t *testing.T) {
	active := &Identity{Name: "personal", FragmentPath: "/cfg/personal.gitconfig"}
	all := []Identity{
		*active,
		{Name: "work", Root: "/home/u/work", FragmentPath: "/cfg/work.gitconfig"},
	}

	got := GlobalBlock(active, all)
	activeAt := strings.Index(got, "personal.gitconfig")
	scopedAt := strings.Index(got, "includeIf")
	if activeAt < 0 || scopedAt < 0 {
		t.Fatalf("missing expected entries:\n%s", got)
	}
	// Later includes win in git, so a directory rule must follow the active one.
	if scopedAt < activeAt {
		t.Errorf("includeIf must come after the active include, or scoping cannot override:\n%s", got)
	}
	if !strings.Contains(got, `gitdir:/home/u/work/`) {
		t.Errorf("gitdir needs a trailing slash to match a tree:\n%s", got)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
