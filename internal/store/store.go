package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const stateFileName = "state.json"
const stateLockFileName = "state.lock"

const (
	lockWaitTimeout = 15 * time.Second
	lockPollDelay   = 50 * time.Millisecond
	staleLockAge    = 10 * time.Minute
)

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

type SettingsPreset struct {
	Tool      Tool      `json:"tool"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type SettingsSync struct {
	Tool      Tool      `json:"tool"`
	Profile   string    `json:"profile"`
	Preset    string    `json:"preset"`
	UpdatedAt time.Time `json:"updated_at"`
}

type State struct {
	Version         int              `json:"version"`
	Defaults        map[Tool]string  `json:"defaults"`
	Profiles        []Profile        `json:"profiles"`
	SettingsPresets []SettingsPreset `json:"settings_presets,omitempty"`
	SettingsSync    []SettingsSync   `json:"settings_sync,omitempty"`
}

type Store struct {
	root string
}

func DefaultRoot() (string, error) {
	if custom := strings.TrimSpace(os.Getenv("PROFILEX_HOME")); custom != "" {
		return custom, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".profilex"), nil
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

func (s *Store) lockPath() string {
	return filepath.Join(s.root, stateLockFileName)
}

func (s *Store) Load() (*State, error) {
	lock, err := s.acquireLock()
	if err != nil {
		return nil, err
	}
	defer s.releaseLock(lock)

	st, err := s.loadUnlocked()
	if err != nil {
		return nil, err
	}
	return st, nil
}

func (s *Store) Save(st *State) error {
	if st == nil {
		return errors.New("state cannot be nil")
	}

	lock, err := s.acquireLock()
	if err != nil {
		return err
	}
	defer s.releaseLock(lock)

	return s.writeStateUnlocked(st)
}

func (s *Store) Update(fn func(*State) error) error {
	if fn == nil {
		return errors.New("update callback cannot be nil")
	}

	lock, err := s.acquireLock()
	if err != nil {
		return err
	}
	defer s.releaseLock(lock)

	st, err := s.loadUnlocked()
	if err != nil {
		return err
	}
	if err := fn(st); err != nil {
		return err
	}
	return s.writeStateUnlocked(st)
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

func ValidatePresetName(name string) error {
	return ValidateProfileName(name)
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

func FindSettingsPreset(st *State, tool Tool, name string) (int, *SettingsPreset) {
	for i := range st.SettingsPresets {
		p := &st.SettingsPresets[i]
		if p.Tool == tool && p.Name == name {
			return i, p
		}
	}
	return -1, nil
}

func FindSettingsSync(st *State, tool Tool, profile string) (int, *SettingsSync) {
	for i := range st.SettingsSync {
		b := &st.SettingsSync[i]
		if b.Tool == tool && b.Profile == profile {
			return i, b
		}
	}
	return -1, nil
}

func defaultState() *State {
	return &State{
		Version:         1,
		Defaults:        map[Tool]string{},
		Profiles:        []Profile{},
		SettingsPresets: []SettingsPreset{},
		SettingsSync:    []SettingsSync{},
	}
}

func normalizeState(st *State) {
	if st.Defaults == nil {
		st.Defaults = map[Tool]string{}
	}
	if st.Profiles == nil {
		st.Profiles = []Profile{}
	}
	if st.SettingsPresets == nil {
		st.SettingsPresets = []SettingsPreset{}
	}
	if st.SettingsSync == nil {
		st.SettingsSync = []SettingsSync{}
	}
	st.Version = 1
	sort.Slice(st.Profiles, func(i, j int) bool {
		if st.Profiles[i].Tool == st.Profiles[j].Tool {
			return st.Profiles[i].Name < st.Profiles[j].Name
		}
		return st.Profiles[i].Tool < st.Profiles[j].Tool
	})
	sort.Slice(st.SettingsPresets, func(i, j int) bool {
		if st.SettingsPresets[i].Tool == st.SettingsPresets[j].Tool {
			return st.SettingsPresets[i].Name < st.SettingsPresets[j].Name
		}
		return st.SettingsPresets[i].Tool < st.SettingsPresets[j].Tool
	})
	sort.Slice(st.SettingsSync, func(i, j int) bool {
		if st.SettingsSync[i].Tool == st.SettingsSync[j].Tool {
			return st.SettingsSync[i].Profile < st.SettingsSync[j].Profile
		}
		return st.SettingsSync[i].Tool < st.SettingsSync[j].Tool
	})
}

func (s *Store) loadUnlocked() (*State, error) {
	b, err := os.ReadFile(s.statePath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return defaultState(), nil
		}
		return nil, err
	}

	st := State{}
	if err := json.Unmarshal(b, &st); err != nil {
		return nil, err
	}
	normalizeState(&st)
	return &st, nil
}

func (s *Store) writeStateUnlocked(st *State) error {
	if st == nil {
		return errors.New("state cannot be nil")
	}
	normalizeState(st)

	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')

	tmp, err := os.CreateTemp(s.root, ".state-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	keepTmp := false
	defer func() {
		if !keepTmp {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, s.statePath()); err != nil {
		if runtime.GOOS != "windows" {
			return err
		}
		if rmErr := os.Remove(s.statePath()); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
			return fmt.Errorf("replace state file: %w", rmErr)
		}
		if err := os.Rename(tmpPath, s.statePath()); err != nil {
			return err
		}
	}
	keepTmp = true
	return nil
}

func (s *Store) acquireLock() (*os.File, error) {
	deadline := time.Now().Add(lockWaitTimeout)
	lockPath := s.lockPath()

	for {
		lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_, _ = fmt.Fprintf(lock, "pid=%d\ntime=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339Nano))
			return lock, nil
		}

		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}

		s.tryRemoveStaleLock(lockPath)
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for state lock: %s", lockPath)
		}
		time.Sleep(lockPollDelay)
	}
}

func (s *Store) releaseLock(lock *os.File) {
	if lock == nil {
		return
	}
	lockPath := lock.Name()
	lockInfo, _ := lock.Stat()
	_ = lock.Close()
	if lockInfo == nil {
		_ = os.Remove(lockPath)
		return
	}
	pathInfo, err := os.Stat(lockPath)
	if err != nil {
		return
	}
	if os.SameFile(lockInfo, pathInfo) {
		_ = os.Remove(lockPath)
	}
}

func (s *Store) tryRemoveStaleLock(lockPath string) {
	info, err := os.Stat(lockPath)
	if err != nil {
		return
	}
	if time.Since(info.ModTime()) <= staleLockAge {
		return
	}
	pid := lockPID(lockPath)
	if pid > 0 && processExists(pid) {
		return
	}
	_ = os.Remove(lockPath)
}

func lockPID(lockPath string) int {
	b, err := os.ReadFile(lockPath)
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "pid=") {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "pid=")))
		if err != nil || pid <= 0 {
			return 0
		}
		return pid
	}
	return 0
}

func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	if errors.Is(err, os.ErrProcessDone) {
		return false
	}
	if runtime.GOOS == "windows" {
		// os.Process.Signal(0) is not reliably supported on windows.
		// Be conservative and avoid deleting a potentially live lock.
		return true
	}
	if errors.Is(err, syscall.EPERM) {
		return true
	}
	return false
}
