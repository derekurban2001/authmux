package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/derekurban/proflex-cli/internal/store"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	m, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestEnsureProfileCreatesAndSetsDefault(t *testing.T) {
	m := newTestManager(t)
	p, created, err := m.EnsureProfile(store.ToolClaude, "personal")
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatalf("expected profile to be created")
	}
	if p.Tool != store.ToolClaude || p.Name != "personal" {
		t.Fatalf("unexpected profile: %+v", p)
	}
	if _, err := os.Stat(p.Dir); err != nil {
		t.Fatalf("expected profile dir to exist: %v", err)
	}

	st, err := m.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := st.Defaults[store.ToolClaude]; got != "personal" {
		t.Fatalf("unexpected default: %q", got)
	}

	_, created2, err := m.EnsureProfile(store.ToolClaude, "personal")
	if err != nil {
		t.Fatal(err)
	}
	if created2 {
		t.Fatalf("expected existing profile not to be created again")
	}
}

func TestSetDefaultAndResolveProfile(t *testing.T) {
	m := newTestManager(t)
	if _, _, err := m.EnsureProfile(store.ToolCodex, "work"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.EnsureProfile(store.ToolCodex, "client"); err != nil {
		t.Fatal(err)
	}

	if err := m.SetDefault(store.ToolCodex, "client"); err != nil {
		t.Fatal(err)
	}

	st, err := m.Load()
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := m.ResolveProfile(st, store.ToolCodex, "")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Name != "client" {
		t.Fatalf("expected default resolve to client, got %s", resolved.Name)
	}

	explicit, err := m.ResolveProfile(st, store.ToolCodex, "work")
	if err != nil {
		t.Fatal(err)
	}
	if explicit.Name != "work" {
		t.Fatalf("expected explicit profile work, got %s", explicit.Name)
	}
}

func TestRenameProfile(t *testing.T) {
	m := newTestManager(t)
	p, _, err := m.EnsureProfile(store.ToolClaude, "old")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.SetDefault(store.ToolClaude, "old"); err != nil {
		t.Fatal(err)
	}
	if err := m.RenameProfile(store.ToolClaude, "old", "new"); err != nil {
		t.Fatal(err)
	}

	st, err := m.Load()
	if err != nil {
		t.Fatal(err)
	}
	if st.Defaults[store.ToolClaude] != "new" {
		t.Fatalf("default should have been moved to new name")
	}
	_, found := store.FindProfile(st, store.ToolClaude, "new")
	if found == nil {
		t.Fatalf("expected renamed profile to exist")
	}
	if _, err := os.Stat(filepath.Join(m.Root(), "profiles", "claude", "new")); err != nil {
		t.Fatalf("renamed profile dir missing: %v", err)
	}
	if _, err := os.Stat(p.Dir); !os.IsNotExist(err) {
		t.Fatalf("old profile dir should be gone")
	}
}

func TestRemoveProfileAndReassignDefault(t *testing.T) {
	m := newTestManager(t)
	if _, _, err := m.EnsureProfile(store.ToolCodex, "a"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.EnsureProfile(store.ToolCodex, "b"); err != nil {
		t.Fatal(err)
	}
	if err := m.SetDefault(store.ToolCodex, "a"); err != nil {
		t.Fatal(err)
	}
	if err := m.RemoveProfile(store.ToolCodex, "a", false); err != nil {
		t.Fatal(err)
	}

	st, err := m.Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, p := store.FindProfile(st, store.ToolCodex, "a"); p != nil {
		t.Fatalf("profile a should be removed")
	}
	if st.Defaults[store.ToolCodex] != "b" {
		t.Fatalf("default should move to remaining profile b, got %q", st.Defaults[store.ToolCodex])
	}
}

func TestRemoveProfilePurgeDeletesDir(t *testing.T) {
	m := newTestManager(t)
	p, _, err := m.EnsureProfile(store.ToolClaude, "trashme")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.RemoveProfile(store.ToolClaude, "trashme", true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p.Dir); !os.IsNotExist(err) {
		t.Fatalf("expected profile dir to be deleted with purge")
	}
}
