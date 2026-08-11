// Package server is the local management API behind `gsw ui`.
//
// It is localhost-only by design. An HTTP server that can rewrite ~/.gitconfig
// and ~/.ssh/config is a serious thing to run on a developer machine, so
// security rests on four rules, all enforced here rather than left to the
// frontend:
//
//  1. Bind 127.0.0.1 exclusively, never 0.0.0.0. Nothing off this machine can
//     open a connection at all.
//  2. Mint a random 256-bit session token at startup, hand it to the browser via
//     the opened URL, and require it as a bearer token on every API request. The
//     token lives in memory and dies with the process.
//  3. Validate the Host and Origin headers on every API request. This is what
//     blocks DNS rebinding: an attacker page that re-points its own hostname at
//     127.0.0.1 still sends its own Origin, and still fails the check.
//  4. Never return token values over the API — only whether one is present.
//
// All handlers delegate to internal/profile and internal/apply, which is what
// keeps the UI and the CLI from drifting apart.
package server

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sriyush/gitswitch/internal/apply"
	"github.com/sriyush/gitswitch/internal/checkup"
	"github.com/sriyush/gitswitch/internal/profile"
	"github.com/sriyush/gitswitch/internal/sshcfg"
	"github.com/sriyush/gitswitch/web"
)

// DefaultPort is the preferred listen port. When it is taken, New falls back to
// an ephemeral one rather than failing: the URL is printed and opened for the
// user either way, so the exact number does not matter.
const DefaultPort = 7842

// Server is a running management API. Use New to construct one.
type Server struct {
	token string
	ln    net.Listener
	http  *http.Server

	mu   sync.Mutex // serialises mutations; two writers would race on profiles.json
	subs map[chan string]struct{}
}

// New binds the listener and mints the session token.
//
// Binding happens here, before Serve, so the caller can print and open the real
// URL without racing the goroutine that starts serving.
func New(port int) (*Server, error) {
	if port == 0 {
		port = DefaultPort
	}

	ln, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		if port == DefaultPort {
			if ln, err = net.Listen("tcp", "127.0.0.1:0"); err != nil {
				return nil, fmt.Errorf("binding a local port: %w", err)
			}
		} else {
			return nil, fmt.Errorf("binding 127.0.0.1:%d: %w", port, err)
		}
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		ln.Close()
		return nil, fmt.Errorf("generating a session token: %w", err)
	}

	s := &Server{
		token: base64.RawURLEncoding.EncodeToString(raw),
		ln:    ln,
		subs:  map[chan string]struct{}{},
	}
	s.http = &http.Server{
		Handler:           s.routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return s, nil
}

// Port is the port actually bound, which may not be the one requested.
func (s *Server) Port() int { return s.ln.Addr().(*net.TCPAddr).Port }

// URL is the address to open in a browser, carrying the session token.
//
// The token travels in the query string because it is the only channel to a
// browser that has not loaded any of our code yet. The frontend's first act is
// to move it into sessionStorage and strip it from the URL via
// history.replaceState, so it does not linger in the address bar or in history
// but does survive a reload of the page. sessionStorage is scoped to the one tab
// and cleared when that tab closes, so the token cannot outlive the window it
// was handed to — which localStorage would not guarantee.
func (s *Server) URL() string {
	return fmt.Sprintf("http://127.0.0.1:%d/?t=%s", s.Port(), s.token)
}

// Serve blocks until the server is closed.
func (s *Server) Serve() error {
	go s.watchStore()
	err := s.http.Serve(s.ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Close stops the server.
func (s *Server) Close() error { return s.http.Close() }

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /api/profiles", s.guard(s.listProfiles))
	mux.Handle("POST /api/profiles", s.guard(s.createProfile))
	mux.Handle("PATCH /api/profiles/{name}", s.guard(s.patchProfile))
	mux.Handle("DELETE /api/profiles/{name}", s.guard(s.deleteProfile))
	mux.Handle("GET /api/profiles/{name}/key", s.guard(s.publicKey))
	mux.Handle("POST /api/switch", s.guard(s.switchProfile))
	mux.Handle("GET /api/doctor", s.guard(s.doctor))
	mux.Handle("GET /api/events", s.guard(s.events))

	// The frontend is served unauthenticated: it is static markup and script with
	// no secrets in it, and it is useless without the token that only the opened
	// URL carries. Requiring auth here would mean the browser could never load
	// the page that knows how to send the header.
	app, err := fs.Sub(web.Assets, "app")
	if err != nil {
		panic("embedded frontend missing: " + err.Error())
	}
	mux.Handle("GET /", http.FileServer(http.FS(app)))

	return mux
}

// guard enforces rules 1-3 for every API request.
func (s *Server) guard(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !loopbackHost(r.Host) {
			// A request that arrived with someone else's hostname in it did not
			// come from a user typing 127.0.0.1 — it came from a page whose DNS
			// was pointed here.
			httpError(w, http.StatusForbidden, "request host is not loopback")
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && !s.allowedOrigin(origin) {
			httpError(w, http.StatusForbidden, "cross-origin request refused")
			return
		}
		if !s.authorized(r) {
			httpError(w, http.StatusUnauthorized, "missing or invalid session token")
			return
		}
		next(w, r)
	})
}

// authorized compares the bearer token in constant time, so a caller cannot
// learn the token one byte at a time from response timing.
func (s *Server) authorized(r *http.Request) bool {
	header := r.Header.Get("Authorization")
	got, ok := strings.CutPrefix(header, "Bearer ")
	if !ok {
		// EventSource cannot set headers, so SSE authenticates by query
		// parameter. The token is no more exposed here than in the opened URL it
		// came from, and the connection never leaves the loopback interface.
		got = r.URL.Query().Get("t")
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(s.token)) == 1
}

func (s *Server) allowedOrigin(origin string) bool {
	port := strconv.Itoa(s.Port())
	for _, host := range []string{"127.0.0.1", "localhost", "[::1]"} {
		if origin == "http://"+host+":"+port {
			return true
		}
	}
	return false
}

func loopbackHost(hostport string) bool {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		host = hostport
	}
	switch host {
	case "127.0.0.1", "localhost", "::1", "[::1]":
		return true
	}
	return false
}

// --- views -----------------------------------------------------------------

// profileView is the wire form of a profile. It is a separate type from
// profile.Profile so that adding a secret-bearing field to the model can never
// silently start serving it: this struct has to be edited too.
type profileView struct {
	Name          string   `json:"name"`
	Username      string   `json:"username"`
	GitName       string   `json:"git_name"`
	GitEmail      string   `json:"git_email"`
	SSHKey        string   `json:"ssh_key,omitempty"`
	HostAlias     string   `json:"host_alias,omitempty"`
	SigningKey    string   `json:"signing_key,omitempty"`
	SigningFormat string   `json:"signing_format,omitempty"`
	Root          string   `json:"root,omitempty"`
	Orgs          []string `json:"orgs,omitempty"`
	Active        bool     `json:"active"`

	// TokenStatus reports whether HTTPS credentials are configured, never what
	// they are. It is "unsupported" until internal/keyring exists — reporting
	// "none" would imply a working feature with nothing stored in it.
	TokenStatus string `json:"token_status"`
}

func view(p *profile.Profile, active string) profileView {
	orgs := p.Orgs
	if orgs == nil {
		orgs = []string{}
	}
	return profileView{
		Name:          p.Name,
		Username:      p.Username,
		GitName:       p.GitName,
		GitEmail:      p.GitEmail,
		SSHKey:        p.SSHKey,
		HostAlias:     p.DefaultHostAlias(),
		SigningKey:    p.SigningKey,
		SigningFormat: p.SigningFormat,
		Root:          p.Root,
		Orgs:          orgs,
		Active:        p.Name == active,
		TokenStatus:   "unsupported",
	}
}

// --- handlers --------------------------------------------------------------

func (s *Server) listProfiles(w http.ResponseWriter, r *http.Request) {
	store, err := profile.Load()
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	views := []profileView{}
	for _, p := range store.List() {
		views = append(views, view(p, store.Active))
	}
	// The home directory is sent so the page can display paths as ~/work rather
	// than /home/you/work. Every path in a profile is stored absolute, because
	// git and ssh read them with no shell to expand a tilde — but a card full of
	// absolute paths is unreadable, and the browser has no other way to know
	// which prefix to fold.
	home, _ := os.UserHomeDir()
	writeJSON(w, http.StatusOK, map[string]any{
		"profiles": views,
		"active":   store.Active,
		"home":     home,
	})
}

type createRequest struct {
	Name          string   `json:"name"`
	Username      string   `json:"username"`
	GitName       string   `json:"git_name"`
	GitEmail      string   `json:"git_email"`
	SSHKey        string   `json:"ssh_key"`
	GenerateKey   bool     `json:"generate_key"`
	SigningKey    string   `json:"signing_key"`
	SigningFormat string   `json:"signing_format"`
	Root          string   `json:"root"`
	Orgs          []string `json:"orgs"`
}

func (s *Server) createProfile(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := decode(r, &req); err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	store, err := profile.Load()
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if req.GitName == "" {
		req.GitName = req.Username
	}
	if req.SigningKey == "" {
		req.SigningFormat = ""
	} else if req.SigningFormat == "" {
		req.SigningFormat = "ssh"
	}

	keyPath := profile.ExpandPath(req.SSHKey)
	if keyPath == "" && req.GenerateKey {
		if keyPath, err = sshcfg.DefaultKeyPath(req.Name); err != nil {
			httpError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if _, err := sshcfg.GenerateKey(keyPath, "gitswitch-"+req.Username); err != nil {
			httpError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	p := &profile.Profile{
		Name:          req.Name,
		Username:      req.Username,
		GitName:       req.GitName,
		GitEmail:      req.GitEmail,
		SSHKey:        keyPath,
		SigningKey:    profile.ExpandPath(req.SigningKey),
		SigningFormat: req.SigningFormat,
		Root:          profile.ExpandPath(req.Root),
		Orgs:          req.Orgs,
		TokenRef:      "keyring://gitswitch/" + req.Name,
	}
	p.HostAlias = p.DefaultHostAlias()

	if err := p.Validate(); err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := store.CheckRoot(p.Name, p.Root); err != nil {
		httpError(w, http.StatusConflict, err.Error())
		return
	}
	if err := store.Add(p); err != nil {
		httpError(w, http.StatusConflict, err.Error())
		return
	}
	if err := s.commit(store); err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, view(p, store.Active))
}

// patchRequest uses pointers so an omitted field is distinguishable from one
// explicitly set to empty. This mirrors `gsw edit`, where flag.Visit gives the
// same distinction: omitted means "leave alone", "" means "clear".
type patchRequest struct {
	Username      *string   `json:"username"`
	GitName       *string   `json:"git_name"`
	GitEmail      *string   `json:"git_email"`
	SSHKey        *string   `json:"ssh_key"`
	SigningKey    *string   `json:"signing_key"`
	SigningFormat *string   `json:"signing_format"`
	Root          *string   `json:"root"`
	Orgs          *[]string `json:"orgs"`
}

func (s *Server) patchProfile(w http.ResponseWriter, r *http.Request) {
	var req patchRequest
	if err := decode(r, &req); err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	store, err := profile.Load()
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	p, err := store.Get(r.PathValue("name"))
	if err != nil {
		httpError(w, http.StatusNotFound, err.Error())
		return
	}

	if req.Username != nil {
		p.Username = *req.Username
	}
	if req.GitName != nil {
		p.GitName = *req.GitName
	}
	if req.GitEmail != nil {
		p.GitEmail = *req.GitEmail
	}
	if req.SSHKey != nil {
		p.SSHKey = profile.ExpandPath(*req.SSHKey)
	}
	if req.SigningKey != nil {
		p.SigningKey = profile.ExpandPath(*req.SigningKey)
		if p.SigningKey == "" {
			p.SigningFormat = ""
		} else if p.SigningFormat == "" {
			p.SigningFormat = "ssh"
		}
	}
	if req.SigningFormat != nil {
		p.SigningFormat = *req.SigningFormat
	}
	if req.Root != nil {
		p.Root = profile.ExpandPath(*req.Root)
	}
	if req.Orgs != nil {
		p.Orgs = *req.Orgs
	}

	if err := p.Validate(); err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := store.CheckRoot(p.Name, p.Root); err != nil {
		httpError(w, http.StatusConflict, err.Error())
		return
	}
	if err := s.commit(store); err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, view(p, store.Active))
}

func (s *Server) deleteProfile(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	store, err := profile.Load()
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	name := r.PathValue("name")
	removed, err := store.Get(name)
	if err != nil {
		httpError(w, http.StatusNotFound, err.Error())
		return
	}
	keptKey := removed.SSHKey
	if err := store.Remove(name); err != nil {
		httpError(w, http.StatusNotFound, err.Error())
		return
	}
	if err := s.commit(store); err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Key material is never deleted here, for the same reason `gsw remove` keeps
	// it: the key may be registered on GitHub or referenced elsewhere, and an
	// irreversible deletion is a far worse surprise than a stale file.
	writeJSON(w, http.StatusOK, map[string]any{"removed": name, "kept_ssh_key": keptKey})
}

func (s *Server) switchProfile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := decode(r, &req); err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	store, err := profile.Load()
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	p, err := store.Get(req.Name)
	if err != nil {
		httpError(w, http.StatusNotFound, err.Error())
		return
	}
	store.Active = p.Name
	if err := s.commit(store); err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, view(p, store.Active))
}

func (s *Server) publicKey(w http.ResponseWriter, r *http.Request) {
	store, err := profile.Load()
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	p, err := store.Get(r.PathValue("name"))
	if err != nil {
		httpError(w, http.StatusNotFound, err.Error())
		return
	}
	if p.SSHKey == "" {
		httpError(w, http.StatusNotFound, "profile has no SSH key")
		return
	}
	// The public half of a keypair is not a secret — it exists to be pasted into
	// GitHub, which is exactly what this endpoint is for. The private key is
	// never read, here or anywhere else in this package.
	pub, err := sshcfg.PublicKey(p.SSHKey)
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": p.Name, "public_key": pub})
}

func (s *Server) doctor(w http.ResponseWriter, r *http.Request) {
	store, err := profile.Load()
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	offline := r.URL.Query().Get("offline") == "1"
	res, err := checkup.Run(store, offline)
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// commit saves the store and rewrites every managed config region, then tells
// connected clients to refetch. Callers must hold s.mu.
func (s *Server) commit(store *profile.Store) error {
	if err := store.Save(); err != nil {
		return err
	}
	if err := apply.Store(store); err != nil {
		return err
	}
	s.broadcast("profiles")
	return nil
}

// --- events ----------------------------------------------------------------

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		httpError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	ch := make(chan string, 1)
	s.mu.Lock()
	s.subs[ch] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.subs, ch)
		s.mu.Unlock()
	}()

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	// A periodic comment keeps the connection from being reaped by an idle
	// timeout somewhere in the stack; SSE ignores lines starting with ':'.
	keepalive := time.NewTicker(30 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-ch:
			fmt.Fprintf(w, "event: %s\ndata: {}\n\n", msg)
			flusher.Flush()
		case <-keepalive.C:
			fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

// broadcast notifies subscribers without blocking. A client that is not keeping
// up already has a pending "go refetch" message, and a second one would tell it
// nothing new — so a full buffer is simply skipped.
func (s *Server) broadcast(event string) {
	for ch := range s.subs {
		select {
		case ch <- event:
		default:
		}
	}
}

// watchStore polls profiles.json and broadcasts when it changes underneath us.
//
// This is what keeps the UI honest when someone runs `gsw switch` in a terminal
// while the page is open. Polling rather than inotify is deliberate: one stat
// per second costs nothing, and it behaves identically across platforms and on
// the atomic rename that Store.Save performs — which is a create, not a write,
// and so is missed by naive watchers.
func (s *Server) watchStore() {
	dir, err := profile.ConfigDir()
	if err != nil {
		return
	}
	path := filepath.Join(dir, "profiles.json")

	var last string
	stamp := func() string {
		fi, err := os.Stat(path)
		if err != nil {
			return ""
		}
		return fmt.Sprintf("%d/%d", fi.ModTime().UnixNano(), fi.Size())
	}
	last = stamp()

	for range time.Tick(time.Second) {
		if now := stamp(); now != last {
			last = now
			s.mu.Lock()
			s.broadcast("profiles")
			s.mu.Unlock()
		}
	}
}

// --- helpers ---------------------------------------------------------------

// decode reads a JSON body, capped so a malformed or hostile request cannot
// make the server allocate without bound, and strict about unknown fields so a
// typo in a field name fails loudly instead of being silently ignored.
func decode(r *http.Request, dst any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("invalid request body: %w", err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func httpError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
