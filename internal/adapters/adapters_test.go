package adapters

import (
	"strings"
	"testing"

	"github.com/derekurban2001/proflex/internal/store"
)

func hasEnvPrefix(env []string, prefix string) bool {
	for _, v := range env {
		if strings.HasPrefix(v, prefix) {
			return true
		}
	}
	return false
}

func TestClaudeCommandEnvironment(t *testing.T) {
	var a Claude
	cmd := a.RunCommand("/tmp/claude-p", []string{"auth", "login"})
	if got := cmd.Args; len(got) < 3 || got[0] != "claude" || got[1] != "auth" || got[2] != "login" {
		t.Fatalf("unexpected run args: %#v", got)
	}
	if !hasEnvPrefix(cmd.Env, "CLAUDE_CONFIG_DIR=/tmp/claude-p") {
		t.Fatalf("expected CLAUDE_CONFIG_DIR env var")
	}
}

func TestClaudeEnvVar(t *testing.T) {
	var a Claude
	if a.EnvVar() != "CLAUDE_CONFIG_DIR" {
		t.Fatalf("unexpected env var: %s", a.EnvVar())
	}
}

func TestCodexCommandEnvironment(t *testing.T) {
	var a Codex
	cmd := a.RunCommand("/tmp/codex-p", []string{"--profile", "deep"})
	if got := cmd.Args; len(got) < 1 || got[0] != "codex" {
		t.Fatalf("unexpected run args: %#v", got)
	}
	if !hasEnvPrefix(cmd.Env, "CODEX_HOME=/tmp/codex-p") {
		t.Fatalf("expected CODEX_HOME env var")
	}
}

func TestCodexEnvVar(t *testing.T) {
	var a Codex
	if a.EnvVar() != "CODEX_HOME" {
		t.Fatalf("unexpected env var: %s", a.EnvVar())
	}
}

func TestGetAdapter(t *testing.T) {
	if _, err := Get(store.ToolClaude); err != nil {
		t.Fatalf("claude adapter should resolve: %v", err)
	}
	if _, err := Get(store.ToolCodex); err != nil {
		t.Fatalf("codex adapter should resolve: %v", err)
	}
	if _, err := Get("other"); err == nil {
		t.Fatalf("unknown adapter should fail")
	}
}
