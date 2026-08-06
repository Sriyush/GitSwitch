// Package profile defines the account identity model shared by the CLI and the
// management server. Everything in gitswitch resolves to a Profile.
package profile

import (
	"fmt"
	"regexp"
)

// Profile is one GitHub account identity, spanning all four layers that git
// authentication actually depends on: commit identity, SSH auth, HTTPS token,
// and commit signing.
type Profile struct {
	// Name is the local handle used on the command line: `gsw switch work`.
	Name string `json:"name"`

	// Username is the GitHub login, used to verify pushes go to the right account.
	Username string `json:"username"`

	// GitName and GitEmail become user.name / user.email.
	GitName  string `json:"git_name"`
	GitEmail string `json:"git_email"`

	// SSHKey is the path to the private key for this account.
	SSHKey string `json:"ssh_key,omitempty"`

	// HostAlias is the Host entry written to ~/.ssh/config, e.g.
	// "github.com-work". Clone URLs are rewritten to use it so the correct key
	// is offered regardless of which profile is globally active.
	HostAlias string `json:"host_alias,omitempty"`

	// SigningKey is the SSH public key or GPG key id used for commit signing.
	// SigningFormat is "ssh" or "openpgp"; empty disables signing.
	SigningKey    string `json:"signing_key,omitempty"`
	SigningFormat string `json:"signing_format,omitempty"`

	// TokenRef points at the OS keyring entry holding the PAT. The token value
	// itself is never stored in this struct or serialized to disk.
	TokenRef string `json:"token_ref,omitempty"`

	// Root is the directory this profile owns. Paths under it are bound to this
	// profile via an includeIf rule, so identity follows the filesystem.
	Root string `json:"root,omitempty"`

	// Orgs lists GitHub owners routed to this profile by `gsw clone`.
	Orgs []string `json:"orgs,omitempty"`
}

var nameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,31}$`)

// Validate reports whether the profile is usable. It deliberately requires only
// the fields needed to make a correctly attributed commit; SSH, token, and
// signing config are optional and surfaced as warnings by `gsw doctor` instead.
func (p *Profile) Validate() error {
	if !nameRE.MatchString(p.Name) {
		return fmt.Errorf("profile name %q must be lowercase alphanumeric, 1-32 chars, and may contain . _ -", p.Name)
	}
	if p.GitEmail == "" {
		return fmt.Errorf("profile %q: git_email is required", p.Name)
	}
	if p.GitName == "" {
		return fmt.Errorf("profile %q: git_name is required", p.Name)
	}
	if p.SigningKey != "" && p.SigningFormat != "ssh" && p.SigningFormat != "openpgp" {
		return fmt.Errorf("profile %q: signing_format must be \"ssh\" or \"openpgp\", got %q", p.Name, p.SigningFormat)
	}
	return nil
}

// DefaultHostAlias is the ~/.ssh/config Host used when none is set explicitly.
func (p *Profile) DefaultHostAlias() string {
	if p.HostAlias != "" {
		return p.HostAlias
	}
	return "github.com-" + p.Name
}

// CloneURL rewrites owner/repo into an SSH URL pinned to this profile's host
// alias, which is what makes the right key get offered during the handshake.
func (p *Profile) CloneURL(owner, repo string) string {
	return fmt.Sprintf("git@%s:%s/%s.git", p.DefaultHostAlias(), owner, repo)
}
