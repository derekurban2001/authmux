package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/derekurban2001/proflex/internal/store"
)

type Status struct {
	LoggedIn bool   `json:"logged_in"`
	Method   string `json:"method,omitempty"`
	Raw      string `json:"raw,omitempty"`
}

type Adapter interface {
	Tool() store.Tool
	Binary() string
	EnvVar() string
	RunCommand(profileDir string, args []string) *exec.Cmd
	Status(ctx context.Context, profileDir string) (Status, error)
}

func Get(tool store.Tool) (Adapter, error) {
	switch tool {
	case store.ToolClaude:
		return Claude{}, nil
	case store.ToolCodex:
		return Codex{}, nil
	default:
		return nil, fmt.Errorf("unsupported tool: %s", tool)
	}
}

func ensureBinary(binary string) error {
	_, err := exec.LookPath(binary)
	if err != nil {
		return fmt.Errorf("%s not found in PATH", binary)
	}
	return nil
}

func runCombined(ctx context.Context, cmd *exec.Cmd) (string, error) {
	b, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(b)), err
}

type Claude struct{}

func (Claude) Tool() store.Tool { return store.ToolClaude }
func (Claude) Binary() string   { return "claude" }
func (Claude) EnvVar() string   { return "CLAUDE_CONFIG_DIR" }

func (c Claude) withEnv(profileDir string, args ...string) *exec.Cmd {
	cmd := exec.Command(c.Binary(), args...)
	cmd.Env = append(os.Environ(), c.EnvVar()+"="+profileDir)
	return cmd
}

func (c Claude) RunCommand(profileDir string, args []string) *exec.Cmd {
	return c.withEnv(profileDir, args...)
}

func (c Claude) Status(ctx context.Context, profileDir string) (Status, error) {
	if err := ensureBinary(c.Binary()); err != nil {
		return Status{}, err
	}
	cmd := exec.CommandContext(ctx, c.Binary(), "auth", "status", "--json")
	cmd.Env = append(os.Environ(), c.EnvVar()+"="+profileDir)
	out, err := runCombined(ctx, cmd)
	if err != nil {
		return Status{}, err
	}
	var parsed struct {
		LoggedIn   bool   `json:"loggedIn"`
		AuthMethod string `json:"authMethod"`
	}
	if jErr := json.Unmarshal([]byte(out), &parsed); jErr != nil {
		return Status{Raw: out}, nil
	}
	return Status{LoggedIn: parsed.LoggedIn, Method: parsed.AuthMethod, Raw: out}, nil
}

type Codex struct{}

func (Codex) Tool() store.Tool { return store.ToolCodex }
func (Codex) Binary() string   { return "codex" }
func (Codex) EnvVar() string   { return "CODEX_HOME" }

func (c Codex) withEnv(profileDir string, args ...string) *exec.Cmd {
	cmd := exec.Command(c.Binary(), args...)
	cmd.Env = append(os.Environ(), c.EnvVar()+"="+profileDir)
	return cmd
}

func (c Codex) RunCommand(profileDir string, args []string) *exec.Cmd {
	return c.withEnv(profileDir, args...)
}

func (c Codex) Status(ctx context.Context, profileDir string) (Status, error) {
	if err := ensureBinary(c.Binary()); err != nil {
		return Status{}, err
	}
	cmd := exec.CommandContext(ctx, c.Binary(), "login", "status")
	cmd.Env = append(os.Environ(), c.EnvVar()+"="+profileDir)
	out, err := runCombined(ctx, cmd)
	low := strings.ToLower(out)
	if strings.Contains(low, "not logged") || strings.Contains(low, "logged out") {
		return Status{LoggedIn: false, Raw: out}, nil
	}
	if err == nil {
		return Status{LoggedIn: true, Raw: out}, nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return Status{LoggedIn: false, Raw: out}, nil
	}
	return Status{}, err
}
