package cli

import "testing"

// The host-alias form is the one that matters most: it is what gitswitch writes
// into remotes, and mis-parsing it made the pre-push guard silently allow
// everything it was meant to catch.
func TestSplitRepo(t *testing.T) {
	cases := []struct {
		in          string
		owner, repo string
		ok          bool
	}{
		{"Sriyush/GitSwitch", "Sriyush", "GitSwitch", true},
		{"git@github.com:Sriyush/GitSwitch.git", "Sriyush", "GitSwitch", true},
		{"git@github.com-sriyush:Sriyush/GitSwitch.git", "Sriyush", "GitSwitch", true},
		{"git@github.com-work-2:acme/api.git", "acme", "api", true},
		{"https://github.com/Sriyush/GitSwitch.git", "Sriyush", "GitSwitch", true},
		{"https://github.com/Sriyush/GitSwitch", "Sriyush", "GitSwitch", true},
		{"ssh://git@github.com/acme/api.git", "acme", "api", true},
		{"  Sriyush/GitSwitch  ", "Sriyush", "GitSwitch", true},

		{"Sriyush", "", "", false},
		{"a/b/c", "", "", false},
		{"", "", "", false},
		{"/", "", "", false},
	}

	for _, c := range cases {
		owner, repo, ok := splitRepo(c.in)
		if ok != c.ok || owner != c.owner || repo != c.repo {
			t.Errorf("splitRepo(%q) = (%q, %q, %v), want (%q, %q, %v)",
				c.in, owner, repo, ok, c.owner, c.repo, c.ok)
		}
	}
}
