package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// ErrNotFound is returned when a named profile does not exist.
var ErrNotFound = errors.New("profile not found")

// Store is the on-disk profile database. It is the single source of truth for
// both the CLI and the management server, so the two can never disagree.
type Store struct {
	Profiles map[string]*Profile `json:"profiles"`
	Active   string              `json:"active"`

	path string
}

// ConfigDir returns ~/.config/gitswitch, honouring XDG_CONFIG_HOME.
func ConfigDir() (string, error) {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, "gitswitch"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "gitswitch"), nil
}

// Load reads the store, returning an empty one if it does not exist yet.
func Load() (*Store, error) {
	dir, err := ConfigDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "profiles.json")

	s := &Store{Profiles: map[string]*Profile{}, path: path}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if s.Profiles == nil {
		s.Profiles = map[string]*Profile{}
	}
	s.path = path
	return s, nil
}

// Save writes the store atomically. Writing to a temp file and renaming means an
// interrupted save can never leave a truncated profile database behind.
func (s *Store) Save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".profiles-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, s.path)
}

// Get returns a profile by name.
func (s *Store) Get(name string) (*Profile, error) {
	p, ok := s.Profiles[name]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrNotFound, name)
	}
	return p, nil
}

// Add stores a new profile, refusing to overwrite an existing one.
func (s *Store) Add(p *Profile) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if _, exists := s.Profiles[p.Name]; exists {
		return fmt.Errorf("profile %q already exists", p.Name)
	}
	s.Profiles[p.Name] = p
	if s.Active == "" {
		s.Active = p.Name
	}
	return nil
}

// Remove deletes a profile, clearing the active pointer if it was active.
func (s *Store) Remove(name string) error {
	if _, ok := s.Profiles[name]; !ok {
		return fmt.Errorf("%w: %q", ErrNotFound, name)
	}
	delete(s.Profiles, name)
	if s.Active == name {
		s.Active = ""
	}
	return nil
}

// List returns profiles sorted by name for stable output.
func (s *Store) List() []*Profile {
	out := make([]*Profile, 0, len(s.Profiles))
	for _, p := range s.Profiles {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ActiveProfile returns the currently active profile, if any.
func (s *Store) ActiveProfile() (*Profile, bool) {
	if s.Active == "" {
		return nil, false
	}
	p, ok := s.Profiles[s.Active]
	return p, ok
}

// ResolveOwner finds the profile that owns a GitHub org/user, which is how
// `gsw clone acme/api` picks an identity without being told.
func (s *Store) ResolveOwner(owner string) (*Profile, bool) {
	for _, p := range s.List() {
		for _, o := range p.Orgs {
			if o == owner {
				return p, true
			}
		}
	}
	for _, p := range s.List() {
		if p.Username == owner {
			return p, true
		}
	}
	return nil, false
}
