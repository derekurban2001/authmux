package store

import (
	"path/filepath"
	"testing"
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
