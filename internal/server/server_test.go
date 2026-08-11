package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// start runs a real server against a throwaway HOME. The listener is real rather
// than httptest's because the security rules are written in terms of the port
// this server bound, and a stub would test different code than ships.
func start(t *testing.T) (*Server, string) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	s, err := New(0)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	go s.Serve()
	t.Cleanup(func() { s.Close() })

	return s, "http://127.0.0.1:" + strconv.Itoa(s.Port())
}

func do(t *testing.T, s *Server, method, url, body string, headers map[string]string) *http.Response {
	t.Helper()
	var rdr *bytes.Reader
	if body == "" {
		rdr = bytes.NewReader(nil)
	} else {
		rdr = bytes.NewReader([]byte(body))
	}
	req, err := http.NewRequest(method, url, rdr)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		if k == "Host" {
			req.Host = v
			continue
		}
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	t.Cleanup(func() { res.Body.Close() })
	return res
}

// The four rules in the package doc are the only thing standing between a web
// page and a rewritten ~/.gitconfig, so each gets a test.
func TestGuardRejectsUnauthorized(t *testing.T) {
	s, base := start(t)

	cases := []struct {
		name    string
		headers map[string]string
		want    int
	}{
		{"no token", map[string]string{"Authorization": ""}, http.StatusUnauthorized},
		{"wrong token", map[string]string{"Authorization": "Bearer not-the-token"}, http.StatusUnauthorized},
		{"token prefix only", map[string]string{"Authorization": "Bearer " + s.token[:8]}, http.StatusUnauthorized},
		{"foreign origin", map[string]string{"Origin": "http://evil.example"}, http.StatusForbidden},
		{"rebound host", map[string]string{"Host": "evil.example"}, http.StatusForbidden},
		{"own origin", map[string]string{"Origin": "http://127.0.0.1:" + strconv.Itoa(s.Port())}, http.StatusOK},
		{"no origin at all", nil, http.StatusOK},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := do(t, s, http.MethodGet, base+"/api/profiles", "", c.headers)
			if res.StatusCode != c.want {
				t.Errorf("status = %d, want %d", res.StatusCode, c.want)
			}
		})
	}
}

// The frontend must load without a token, since it is the thing that knows how
// to send one. It carries no secrets, so this is safe — but it should stay
// deliberate rather than accidental.
func TestStaticAssetsNeedNoToken(t *testing.T) {
	_, base := start(t)
	res, err := http.Get(base + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET / = %d, want 200", res.StatusCode)
	}
}

func TestProfileLifecycle(t *testing.T) {
	s, base := start(t)

	res := do(t, s, http.MethodPost, base+"/api/profiles",
		`{"name":"work","username":"you-acme","git_email":"you@acme.com","root":"`+t.TempDir()+`","orgs":["acme"]}`, nil)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create = %d, want 201", res.StatusCode)
	}

	var listed struct {
		Profiles []profileView `json:"profiles"`
		Active   string        `json:"active"`
	}
	res = do(t, s, http.MethodGet, base+"/api/profiles", "", nil)
	if err := json.NewDecoder(res.Body).Decode(&listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Profiles) != 1 || listed.Profiles[0].Name != "work" {
		t.Fatalf("list = %+v, want one profile named work", listed.Profiles)
	}

	// A token value must never appear on the wire, whatever else changes about
	// the model.
	if listed.Profiles[0].TokenStatus != "unsupported" {
		t.Errorf("token_status = %q, want %q", listed.Profiles[0].TokenStatus, "unsupported")
	}

	// Switching through the API must write the same config the CLI writes.
	res = do(t, s, http.MethodPost, base+"/api/switch", `{"name":"work"}`, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("switch = %d, want 200", res.StatusCode)
	}
	gitconfig, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".gitconfig"))
	if err != nil {
		t.Fatalf("reading ~/.gitconfig: %v", err)
	}
	if !bytes.Contains(gitconfig, []byte("gitswitch managed")) {
		t.Errorf("~/.gitconfig has no managed block after a switch:\n%s", gitconfig)
	}

	res = do(t, s, http.MethodPatch, base+"/api/profiles/work", `{"root":""}`, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("patch = %d, want 200", res.StatusCode)
	}
	var patched profileView
	if err := json.NewDecoder(res.Body).Decode(&patched); err != nil {
		t.Fatal(err)
	}
	if patched.Root != "" {
		t.Errorf("root = %q after clearing it, want empty", patched.Root)
	}

	res = do(t, s, http.MethodDelete, base+"/api/profiles/work", "", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("delete = %d, want 200", res.StatusCode)
	}
}

func TestCreateRejectsBadInput(t *testing.T) {
	s, base := start(t)
	root := t.TempDir()

	if res := do(t, s, http.MethodPost, base+"/api/profiles",
		`{"name":"work","username":"w","git_email":"w@x.com","root":"`+root+`"}`, nil); res.StatusCode != http.StatusCreated {
		t.Fatalf("seed = %d, want 201", res.StatusCode)
	}

	cases := []struct {
		name, body string
		want       int
	}{
		{"invalid name", `{"name":"Bad Name","username":"x","git_email":"x@y.com"}`, http.StatusBadRequest},
		{"no email", `{"name":"noemail","username":"x"}`, http.StatusBadRequest},
		{"duplicate", `{"name":"work","username":"x","git_email":"x@y.com"}`, http.StatusConflict},
		{"nested root", `{"name":"nested","username":"x","git_email":"x@y.com","root":"` + filepath.Join(root, "sub") + `"}`, http.StatusConflict},
		{"unknown field", `{"name":"typo","username":"x","git_email":"x@y.com","toke":"secret"}`, http.StatusBadRequest},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := do(t, s, http.MethodPost, base+"/api/profiles", c.body, nil)
			if res.StatusCode != c.want {
				t.Errorf("status = %d, want %d", res.StatusCode, c.want)
			}
		})
	}
}
