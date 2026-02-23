package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestValidateProfileName(t *testing.T) {
	ok := []string{"work", "personal-1", "client.a", "x_y", "A1"}
	for _, v := range ok {
		if err := ValidateProfileName(v); err != nil {
			t.Fatalf("expected valid name %q: %v", v, err)
		}
	}
	bad := []string{"", " space", "a/b", "🔥", "name with space"}
	for _, v := range bad {
		if err := ValidateProfileName(v); err == nil {
			t.Fatalf("expected invalid name %q", v)
		}
	}
}

func TestStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	st := &State{
		Defaults: map[Tool]string{ToolClaude: "personal"},
		Profiles: []Profile{{Tool: ToolClaude, Name: "personal", Dir: filepath.Join(dir, "profiles", "claude", "personal")}},
	}
	if err := s.Save(st); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.Defaults[ToolClaude]; got != "personal" {
		t.Fatalf("unexpected default: %s", got)
	}
	if len(loaded.Profiles) != 1 {
		t.Fatalf("unexpected profiles count: %d", len(loaded.Profiles))
	}
}

func TestStoreUpdateSerializesConcurrentWrites(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	const workers = 24
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := s.Update(func(st *State) error {
				name := fmt.Sprintf("p-%02d", i)
				st.Profiles = append(st.Profiles, Profile{
					Tool: ToolClaude,
					Name: name,
					Dir:  ProfileDir(dir, ToolClaude, name),
				})
				return nil
			})
			if err != nil {
				t.Errorf("update %d failed: %v", i, err)
			}
		}()
	}
	wg.Wait()

	loaded, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Profiles) != workers {
		t.Fatalf("expected %d profiles, got %d", workers, len(loaded.Profiles))
	}
}

func TestStoreUpdateRemovesStaleLock(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	lockPath := filepath.Join(dir, stateLockFileName)
	if err := os.WriteFile(lockPath, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-(staleLockAge + time.Second))
	if err := os.Chtimes(lockPath, old, old); err != nil {
		t.Fatal(err)
	}

	if err := s.Update(func(st *State) error {
		st.Defaults[ToolClaude] = "work"
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	loaded, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Defaults[ToolClaude] != "work" {
		t.Fatalf("expected default to be saved after stale lock cleanup")
	}
}
