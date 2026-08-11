package profile

import (
	"path/filepath"
	"testing"
)

func testStore(t *testing.T, profiles ...*Profile) *Store {
	t.Helper()
	s := &Store{Profiles: map[string]*Profile{}, path: filepath.Join(t.TempDir(), "profiles.json")}
	for _, p := range profiles {
		if err := s.Add(p); err != nil {
			t.Fatalf("seeding %q: %v", p.Name, err)
		}
	}
	return s
}

// GitHub owners are case-insensitive, so a case-sensitive compare here does not
// just inconvenience `gsw clone` — it makes the pre-push guard skip a repo it
// should have judged, and a guard that silently passes is worse than none.
func TestResolveOwnerIsCaseInsensitive(t *testing.T) {
	s := testStore(t,
		&Profile{Name: "work", Username: "you-acme", GitName: "You", GitEmail: "you@acme.com", Orgs: []string{"Acme"}},
		&Profile{Name: "personal", Username: "Sriyush", GitName: "S", GitEmail: "me@example.com"},
	)

	cases := []struct {
		owner string
		want  string
	}{
		{"Acme", "work"},
		{"acme", "work"},
		{"ACME", "work"},
		{"Sriyush", "personal"},
		{"sriyush", "personal"},
		{"nobody", ""},
	}

	for _, c := range cases {
		p, ok := s.ResolveOwner(c.owner)
		switch {
		case c.want == "" && ok:
			t.Errorf("ResolveOwner(%q) matched %q, want no match", c.owner, p.Name)
		case c.want != "" && !ok:
			t.Errorf("ResolveOwner(%q) found nothing, want %q", c.owner, c.want)
		case c.want != "" && ok && p.Name != c.want:
			t.Errorf("ResolveOwner(%q) = %q, want %q", c.owner, p.Name, c.want)
		}
	}
}

// An explicit --orgs claim outranks a username that happens to match, since the
// user stated the first and only implied the second.
func TestResolveOwnerPrefersExplicitOrgs(t *testing.T) {
	s := testStore(t,
		&Profile{Name: "byname", Username: "acme", GitName: "A", GitEmail: "a@x.com"},
		&Profile{Name: "byorg", Username: "someone", GitName: "B", GitEmail: "b@x.com", Orgs: []string{"acme"}},
	)
	p, ok := s.ResolveOwner("acme")
	if !ok || p.Name != "byorg" {
		t.Fatalf("ResolveOwner(acme) = %v/%v, want byorg", p, ok)
	}
}

func TestCheckRoot(t *testing.T) {
	s := testStore(t,
		&Profile{Name: "work", Username: "w", GitName: "W", GitEmail: "w@x.com", Root: "/home/me/work"},
	)

	cases := []struct {
		name, root string
		wantErr    bool
	}{
		{"new", "/home/me/personal", false},
		{"new", "", false},
		{"new", "/home/me/work2", false},          // sibling, not nested
		{"new", "/home/me/work", true},            // identical
		{"new", "/home/me/work/sub", true},        // inside an existing root
		{"new", "/home/me", true},                 // contains an existing root
		{"new", "/home/me/work/", true},           // trailing slash is the same dir
		{"work", "/home/me/work/anywhere", false}, // a profile never conflicts with itself
	}

	for _, c := range cases {
		err := s.CheckRoot(c.name, c.root)
		if (err != nil) != c.wantErr {
			t.Errorf("CheckRoot(%q, %q) error = %v, wantErr %v", c.name, c.root, err, c.wantErr)
		}
	}
}

func TestRemoveClearsActive(t *testing.T) {
	s := testStore(t, &Profile{Name: "solo", Username: "s", GitName: "S", GitEmail: "s@x.com"})
	if s.Active != "solo" {
		t.Fatalf("Active = %q, want the first added profile", s.Active)
	}
	if err := s.Remove("solo"); err != nil {
		t.Fatal(err)
	}
	if s.Active != "" {
		t.Errorf("Active = %q after removing it, want empty", s.Active)
	}
}
