// Package keyring stores GitHub tokens in the OS credential store.
//
// Tokens are never written to profiles.json or any file gitswitch manages. The
// profile holds only a TokenRef ("keyring://gitswitch/<name>"); the secret is
// resolved on demand.
//
// Planned surface:
//
//	Set(ref, token) / Get(ref) / Delete(ref)
//	Configure(profile) - point git's credential helper at the profile's token.
//
// Backends: libsecret via D-Bus on Linux (this machine runs Fedora with
// gnome-keyring), Keychain on macOS, Credential Manager on Windows.
// github.com/zalando/go-keyring covers all three with no cgo.
package keyring
