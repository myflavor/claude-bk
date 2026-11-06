// Package provider loads backup target configuration from ~/.claude-bk.
package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const configDirName = ".claude-bk"

// Config is one provider definition.
type Config struct {
	Type     string `json:"type"`
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port,omitempty"`
	Share    string `json:"share,omitempty"`
	User     string `json:"user,omitempty"`
	Password string `json:"password,omitempty"`
	Root     string `json:"root,omitempty"`
	Path     string `json:"path,omitempty"`
}

// dir returns the provider config directory.
func dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, configDirName), nil
}

// Load reads and validates a named provider.
func Load(name string) (*Config, error) {
	dir, err := dir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, name+".json")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("provider %q not found: %s", name, unavailable(dir))
		}
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid JSON in %s: %w", path, err)
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &cfg, nil
}

func (c *Config) validate() error {
	switch c.Type {
	case "smb":
		required := []struct {
			name  string
			value string
		}{
			{"host", c.Host},
			{"share", c.Share},
			{"user", c.User},
			{"password", c.Password},
		}
		for _, f := range required {
			if f.value == "" {
				return fmt.Errorf("missing required field %q for type smb", f.name)
			}
		}
		if c.Port == 0 {
			c.Port = 445
		}
	case "local":
		if c.Path == "" {
			return fmt.Errorf("missing required field \"path\" for type local")
		}
		if !filepath.IsAbs(c.Path) {
			return fmt.Errorf("path must be absolute: %s", c.Path)
		}
	default:
		return fmt.Errorf("unknown type %q (expected \"smb\" or \"local\")", c.Type)
	}
	return nil
}

func unavailable(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "no providers configured in " + dir
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".json" {
			names = append(names, strings.TrimSuffix(e.Name(), ".json"))
		}
	}
	if len(names) == 0 {
		return "no providers configured in " + dir
	}
	return fmt.Sprintf("available in %s: %v", dir, names)
}
