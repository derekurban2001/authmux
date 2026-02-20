package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const stateFileName = "state.json"

var profileNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)

type Tool string

const (
	ToolClaude Tool = "claude"
	ToolCodex  Tool = "codex"
)

var SupportedTools = []Tool{ToolClaude, ToolCodex}

type Profile struct {
	Tool      Tool      `json:"tool"`
	Name      string    `json:"name"`
	Dir       string    `json:"dir"`
	CreatedAt time.Time `json:"created_at"`
}

type State struct {
	Version  int             `json:"version"`
	Defaults map[Tool]string `json:"defaults"`
	Profiles []Profile       `json:"profiles"`
}

type Store struct {
	root string
}

func DefaultRoot() (string, error) {
	if custom := strings.TrimSpace(os.Getenv("AUTHMUX_HOME")); custom != "" {
		return custom, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".authmux"), nil
}

func New(root string) (*Store, error) {
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("store root cannot be empty")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return &Store{root: root}, nil
}

func (s *Store) Root() string {
	return s.root
}

func (s *Store) statePath() string {
	return filepath.Join(s.root, stateFileName)
}

func (s *Store) Load() (*State, error) {
	path := s.statePath()
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &State{
				Version:  1,
				Defaults: map[Tool]string{},
				Profiles: []Profile{},
			}, nil
		}
		return nil, err
	}
	var st State
	if err := json.Unmarshal(b, &st); err != nil {
		return nil, err
	}
	if st.Defaults == nil {
		st.Defaults = map[Tool]string{}
	}
	if st.Profiles == nil {
		st.Profiles = []Profile{}
	}
	if st.Version == 0 {
		st.Version = 1
	}
	sort.Slice(st.Profiles, func(i, j int) bool {
		if st.Profiles[i].Tool == st.Profiles[j].Tool {
			return st.Profiles[i].Name < st.Profiles[j].Name
		}
		return st.Profiles[i].Tool < st.Profiles[j].Tool
	})
	return &st, nil
}

func (s *Store) Save(st *State) error {
	if st == nil {
		return errors.New("state cannot be nil")
	}
	if st.Defaults == nil {
		st.Defaults = map[Tool]string{}
	}
	if st.Profiles == nil {
		st.Profiles = []Profile{}
	}
	st.Version = 1
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(s.statePath(), b, 0o644)
}

func IsSupportedTool(raw string) (Tool, bool) {
	t := Tool(strings.ToLower(strings.TrimSpace(raw)))
	for _, s := range SupportedTools {
		if s == t {
			return t, true
		}
	}
	return "", false
}

func ValidateProfileName(name string) error {
	if !profileNamePattern.MatchString(name) {
		return fmt.Errorf("invalid profile name %q (allowed: letters, digits, ., _, - ; max 64 chars)", name)
	}
	return nil
}

func ProfileDir(root string, tool Tool, name string) string {
	return filepath.Join(root, "profiles", string(tool), name)
}

func FindProfile(st *State, tool Tool, name string) (int, *Profile) {
	for i := range st.Profiles {
		p := &st.Profiles[i]
		if p.Tool == tool && p.Name == name {
			return i, p
		}
	}
	return -1, nil
}

func DefaultProfile(st *State, tool Tool) (string, bool) {
	v, ok := st.Defaults[tool]
	if !ok || strings.TrimSpace(v) == "" {
		return "", false
	}
	return v, true
}
