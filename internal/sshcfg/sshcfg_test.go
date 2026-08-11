package sshcfg

import (
	"strings"
	"testing"
)

// IdentitiesOnly is the load-bearing line in the whole file. Without it ssh
// offers every key the agent holds and GitHub authenticates as whichever one
// matches first — the most common cause of a silent wrong-account push, because
// the push succeeds. A test guards it against a tidy-up that drops it.
func TestRenderPinsIdentity(t *testing.T) {
	out := Render([]Host{{Alias: "github.com-work", Key: "/home/me/.ssh/id_ed25519_work"}})

	for _, want := range []string{
		"Host github.com-work",
		"HostName github.com",
		"User git",
		"IdentityFile /home/me/.ssh/id_ed25519_work",
		"IdentitiesOnly yes",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Render() missing %q:\n%s", want, out)
		}
	}
}

func TestRenderSeparatesHosts(t *testing.T) {
	out := Render([]Host{
		{Alias: "github.com-a", Key: "/keys/a"},
		{Alias: "github.com-b", Key: "/keys/b"},
	})
	if strings.Count(out, "Host github.com-") != 2 {
		t.Fatalf("expected two Host entries:\n%s", out)
	}
	// ssh parses by indentation under a Host line; two Host stanzas running
	// together with no blank line still parse, but the file becomes unreadable
	// for the user who has to trust what we wrote into it.
	if !strings.Contains(out, "\n\nHost github.com-b") {
		t.Errorf("expected a blank line between stanzas:\n%s", out)
	}
}

func TestRenderOmitsIdentityFileWhenNoKey(t *testing.T) {
	out := Render([]Host{{Alias: "github.com-x"}})
	if strings.Contains(out, "IdentityFile") {
		t.Errorf("no key configured, but IdentityFile was written:\n%s", out)
	}
	if !strings.Contains(out, "IdentitiesOnly yes") {
		t.Errorf("IdentitiesOnly must still be set:\n%s", out)
	}
}

func TestVerifyParsesGitHubGreeting(t *testing.T) {
	// GitHub grants no shell, so ssh always exits non-zero here; the login has to
	// come from the banner rather than the exit status.
	m := greeting.FindStringSubmatch("Hi sriyush! You've successfully authenticated, but GitHub does not provide shell access.")
	if m == nil {
		t.Fatal("greeting did not match GitHub's banner")
	}
	if m[1] != "sriyush" {
		t.Errorf("parsed login %q, want %q", m[1], "sriyush")
	}
}
