package authmux

import "testing"

func TestParseTool(t *testing.T) {
	if _, err := parseTool("claude"); err != nil {
		t.Fatalf("claude should parse: %v", err)
	}
	if _, err := parseTool("codex"); err != nil {
		t.Fatalf("codex should parse: %v", err)
	}
	if _, err := parseTool("invalid"); err == nil {
		t.Fatalf("invalid tool should fail")
	}
}

func TestNoCommandAliases(t *testing.T) {
	cmd := NewRootCmd()
	names := map[string]bool{
		"add": true, "list": true, "use": true, "run": true, "status": true,
		"logout": true, "rename": true, "remove": true, "shim": true, "doctor": true,
	}
	for _, c := range cmd.Commands() {
		if !names[c.Name()] {
			continue
		}
		if len(c.Aliases) > 0 {
			t.Fatalf("command %s should not have aliases, got %v", c.Name(), c.Aliases)
		}
	}
}
