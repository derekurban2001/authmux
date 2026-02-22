package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"

	"github.com/derekurban/proflex-cli/internal/adapters"
	"github.com/derekurban/proflex-cli/internal/store"
)

type ExitCodeError struct {
	Code int
}

func (e ExitCodeError) Error() string {
	return fmt.Sprintf("process exited with code %d", e.Code)
}

type StatusRow struct {
	Profile store.Profile   `json:"profile"`
	Status  adapters.Status `json:"status"`
	Error   string          `json:"error,omitempty"`
}

type Manager struct {
	store *store.Store
}

func NewManager(root string) (*Manager, error) {
	s, err := store.New(root)
	if err != nil {
		return nil, err
	}
	return &Manager{store: s}, nil
}

func NewDefaultManager() (*Manager, error) {
	root, err := store.DefaultRoot()
	if err != nil {
		return nil, err
	}
	return NewManager(root)
}

func (m *Manager) Root() string {
	return m.store.Root()
}

func (m *Manager) Load() (*store.State, error) {
	return m.store.Load()
}

func (m *Manager) Save(st *store.State) error {
	return m.store.Save(st)
}

func (m *Manager) EnsureProfile(tool store.Tool, name string) (store.Profile, bool, error) {
	if err := store.ValidateProfileName(name); err != nil {
		return store.Profile{}, false, err
	}
	st, err := m.Load()
	if err != nil {
		return store.Profile{}, false, err
	}
	if _, existing := store.FindProfile(st, tool, name); existing != nil {
		return *existing, false, nil
	}
	dir := store.ProfileDir(m.Root(), tool, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return store.Profile{}, false, err
	}
	p := store.Profile{Tool: tool, Name: name, Dir: dir, CreatedAt: time.Now().UTC()}
	st.Profiles = append(st.Profiles, p)
	sort.Slice(st.Profiles, func(i, j int) bool {
		if st.Profiles[i].Tool == st.Profiles[j].Tool {
			return st.Profiles[i].Name < st.Profiles[j].Name
		}
		return st.Profiles[i].Tool < st.Profiles[j].Tool
	})
	if _, ok := st.Defaults[tool]; !ok {
		st.Defaults[tool] = name
	}
	if err := m.Save(st); err != nil {
		return store.Profile{}, false, err
	}
	return p, true, nil
}

func (m *Manager) GetProfile(st *store.State, tool store.Tool, name string) (store.Profile, error) {
	_, p := store.FindProfile(st, tool, name)
	if p == nil {
		return store.Profile{}, fmt.Errorf("profile not found: %s/%s", tool, name)
	}
	return *p, nil
}

func (m *Manager) ResolveProfile(st *store.State, tool store.Tool, profileOptional string) (store.Profile, error) {
	if profileOptional != "" {
		return m.GetProfile(st, tool, profileOptional)
	}
	name, ok := store.DefaultProfile(st, tool)
	if !ok {
		return store.Profile{}, fmt.Errorf("no default profile set for %s", tool)
	}
	return m.GetProfile(st, tool, name)
}

func (m *Manager) SetDefault(tool store.Tool, name string) error {
	st, err := m.Load()
	if err != nil {
		return err
	}
	if _, p := store.FindProfile(st, tool, name); p == nil {
		return fmt.Errorf("profile not found: %s/%s", tool, name)
	}
	st.Defaults[tool] = name
	return m.Save(st)
}

func (m *Manager) RenameProfile(tool store.Tool, oldName, newName string) error {
	if err := store.ValidateProfileName(newName); err != nil {
		return err
	}
	st, err := m.Load()
	if err != nil {
		return err
	}
	idx, p := store.FindProfile(st, tool, oldName)
	if p == nil {
		return fmt.Errorf("profile not found: %s/%s", tool, oldName)
	}
	if _, exists := store.FindProfile(st, tool, newName); exists != nil {
		return fmt.Errorf("target profile already exists: %s/%s", tool, newName)
	}
	newDir := store.ProfileDir(m.Root(), tool, newName)
	if err := os.MkdirAll(filepath.Dir(newDir), 0o755); err != nil {
		return err
	}
	if err := os.Rename(p.Dir, newDir); err != nil {
		return err
	}
	st.Profiles[idx].Name = newName
	st.Profiles[idx].Dir = newDir
	if st.Defaults[tool] == oldName {
		st.Defaults[tool] = newName
	}
	return m.Save(st)
}

func (m *Manager) RemoveProfile(tool store.Tool, name string, purge bool) error {
	st, err := m.Load()
	if err != nil {
		return err
	}
	idx, p := store.FindProfile(st, tool, name)
	if p == nil {
		return fmt.Errorf("profile not found: %s/%s", tool, name)
	}
	if purge {
		if err := os.RemoveAll(p.Dir); err != nil {
			return err
		}
	}
	st.Profiles = append(st.Profiles[:idx], st.Profiles[idx+1:]...)
	if st.Defaults[tool] == name {
		delete(st.Defaults, tool)
		for _, prof := range st.Profiles {
			if prof.Tool == tool {
				st.Defaults[tool] = prof.Name
				break
			}
		}
	}
	return m.Save(st)
}

func (m *Manager) RunTool(ctx context.Context, profile store.Profile, args []string) error {
	adapter, err := adapters.Get(profile.Tool)
	if err != nil {
		return err
	}
	cmd := adapter.RunCommand(profile.Dir, args)
	return runInteractive(ctx, cmd)
}

func (m *Manager) StatusForProfile(ctx context.Context, profile store.Profile) (adapters.Status, error) {
	adapter, err := adapters.Get(profile.Tool)
	if err != nil {
		return adapters.Status{}, err
	}
	return adapter.Status(ctx, profile.Dir)
}

func (m *Manager) StatusRows(ctx context.Context, filterTool *store.Tool) ([]StatusRow, error) {
	st, err := m.Load()
	if err != nil {
		return nil, err
	}
	rows := []StatusRow{}
	for _, p := range st.Profiles {
		if filterTool != nil && p.Tool != *filterTool {
			continue
		}
		ctxOne, cancel := context.WithTimeout(ctx, 8*time.Second)
		status, sErr := m.StatusForProfile(ctxOne, p)
		cancel()
		row := StatusRow{Profile: p, Status: status}
		if sErr != nil {
			row.Error = sErr.Error()
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func runInteractive(ctx context.Context, cmd *exec.Cmd) error {
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		return ctx.Err()
	case err := <-done:
		if err == nil {
			return nil
		}
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ExitCodeError{Code: ee.ExitCode()}
		}
		return err
	}
}
