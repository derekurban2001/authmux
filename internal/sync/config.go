package sync

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Provider   string `json:"provider"`
	Directory  string `json:"directory"`
	Machine    string `json:"machine"`
	AutoExport bool   `json:"autoExport"`
}

func ConfigPath(root string) string {
	return filepath.Join(root, "sync.json")
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

func Save(path string, c *Config) error {
	if c == nil {
		return errors.New("sync config cannot be nil")
	}
	if err := c.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}

func (c *Config) Validate() error {
	if c == nil {
		return errors.New("sync config cannot be nil")
	}
	provider := strings.ToLower(strings.TrimSpace(c.Provider))
	if provider == "" {
		return errors.New("provider is required")
	}
	if provider != "syncthing" {
		return fmt.Errorf("unsupported provider %q (supported: syncthing)", c.Provider)
	}
	if strings.TrimSpace(c.Directory) == "" {
		return errors.New("directory is required")
	}
	if strings.TrimSpace(c.Machine) == "" {
		return errors.New("machine is required")
	}
	return nil
}

func (c *Config) BundleFilename() string {
	machine := sanitizeName(c.Machine)
	if machine == "" {
		machine = "machine"
	}
	return fmt.Sprintf("local-unified-usage.%s.json", machine)
}

func sanitizeName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	repl := func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}
	out := strings.Map(repl, s)
	out = strings.Trim(out, "-")
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	return out
}
