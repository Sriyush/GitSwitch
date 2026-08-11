package apply

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sriyush/gitswitch/internal/profile"
)

// tempHome points every path this package writes at a throwaway directory, so a
// test can never touch the developer's real ~/.gitconfig.
func tempHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	return home
}

func TestStoreWritesAndPrunes(t *testing.T) {
	home := tempHome(t)

	store, err := profile.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []*profile.Profile{
		{Name: "work", Username: "w", GitName: "W", GitEmail: "w@acme.com", SSHKey: filepath.Join(home, "k1")},
		{Name: "personal", Username: "p", GitName: "P", GitEmail: "p@me.com"},
	} {
		if err := store.Add(p); err != nil {
			t.Fatal(err)
		}
	}
	if err := Store(store); err != nil {
		t.Fatalf("Store: %v", err)
	}

	fragDir := filepath.Join(home, ".config", "gitswitch", "profiles")
	for _, name := range []string{"work.gitconfig", "personal.gitconfig"} {
		if _, err := os.Stat(filepath.Join(fragDir, name)); err != nil {
			t.Errorf("expected fragment %s: %v", name, err)
		}
	}

	// A file the user put here themselves is not ours to delete, however tidy it
	// would be to remove it.
	handwritten := filepath.Join(fragDir, "notes.txt")
	if err := os.WriteFile(handwritten, []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := store.Remove("personal"); err != nil {
		t.Fatal(err)
	}
	if err := Store(store); err != nil {
		t.Fatalf("Store after remove: %v", err)
	}

	if _, err := os.Stat(filepath.Join(fragDir, "personal.gitconfig")); !os.IsNotExist(err) {
		t.Errorf("stale fragment survived removal (err = %v)", err)
	}
	if _, err := os.Stat(filepath.Join(fragDir, "work.gitconfig")); err != nil {
		t.Errorf("live fragment was pruned: %v", err)
	}
	if _, err := os.Stat(handwritten); err != nil {
		t.Errorf("a file we did not generate was deleted: %v", err)
	}
}

// The managed block is fenced precisely so that everything around it survives.
func TestStorePreservesUserGitconfig(t *testing.T) {
	home := tempHome(t)

	gitconfig := filepath.Join(home, ".gitconfig")
	original := "[core]\n\teditor = nvim\n"
	if err := os.WriteFile(gitconfig, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	store, err := profile.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Add(&profile.Profile{Name: "solo", Username: "s", GitName: "S", GitEmail: "s@x.com"}); err != nil {
		t.Fatal(err)
	}
	if err := Store(store); err != nil {
		t.Fatal(err)
	}

	out, err := os.ReadFile(gitconfig)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), original) {
		t.Errorf("user content was not preserved:\n%s", out)
	}
	if !strings.Contains(string(out), "# >>> gitswitch managed >>>") {
		t.Errorf("managed block missing:\n%s", out)
	}
}
