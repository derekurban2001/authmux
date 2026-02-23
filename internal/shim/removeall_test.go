package shim

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/derekurban/profilex-cli/internal/store"
)

func TestRemoveAllOnlyDeletesManagedShims(t *testing.T) {
	dir := t.TempDir()
	_, _ = Install(dir, store.Profile{Tool: store.ToolClaude, Name: "work"}, "profilex")
	_, _ = Install(dir, store.Profile{Tool: store.ToolCodex, Name: "client"}, "profilex")

	foreign := filepath.Join(dir, "claude-foreign")
	if err := os.WriteFile(foreign, []byte("#!/usr/bin/env bash\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	removed, err := RemoveAll(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 2 {
		t.Fatalf("expected 2 removed shims, got %d", len(removed))
	}

	if _, err := os.Stat(foreign); err != nil {
		t.Fatalf("foreign file should remain")
	}
}
