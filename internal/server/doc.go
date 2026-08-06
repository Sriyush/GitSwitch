// Package server is the local management API behind `gsw ui`.
//
// It is localhost-only by design. Security rests on four rules:
//
//  1. Bind 127.0.0.1 exclusively, never 0.0.0.0.
//  2. Mint a random 256-bit session token at startup, hand it to the browser via
//     the opened URL, and require it as a bearer token on every request. The
//     token lives in memory and dies with the process.
//  3. Validate the Origin header on every request, which is what blocks a
//     DNS-rebinding attack from a page the user happens to have open.
//  4. Never return token values over the API - only whether one is present and
//     when it expires.
//
// Planned routes:
//
//	GET    /api/profiles        POST /api/profiles
//	PATCH  /api/profiles/{name} DELETE /api/profiles/{name}
//	POST   /api/switch          GET  /api/doctor
//	GET    /api/events          - SSE, so the UI updates live when the identity
//	                              is switched from a terminal.
//
// The built frontend is embedded with go:embed, keeping distribution to a single
// binary. All handlers delegate to internal/profile, so the UI and CLI cannot
// drift apart.
package server
