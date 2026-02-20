package shim

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/derekurban2001/authmux/internal/store"
)

func TestInstallAndRemove(t *testing.T) {
	dir := t.TempDir()
	p := store.Profile{Tool: store.ToolClaude, Name: "work"}
	path, err := Install(dir, p, "authmux")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("shim not created: %v", err)
	}
	if filepath.Base(path) != "claude-work" {
		t.Fatalf("unexpected shim name: %s", filepath.Base(path))
	}
	if err := Remove(dir, p); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("shim should be removed")
	}
}
