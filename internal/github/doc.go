// Package github talks to the GitHub REST API.
//
// Planned surface:
//
//	DeviceFlow()          - OAuth device authorization, so `gsw add` never asks
//	    anyone to paste a PAT by hand.
//	UploadSSHKey(token, pubkey)
//	UploadSigningKey(token, pubkey)
//	WhoAmI(token)         - GET /user, to confirm a token belongs to the account
//	    the profile claims.
//	TokenScopes(token)    - read X-OAuth-Scopes and the expiry header so doctor
//	    can warn before a fine-grained PAT lapses.
//
// The standard library's net/http is sufficient; no SDK dependency is warranted
// for this handful of endpoints.
package github
