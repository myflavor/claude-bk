// Package project maps a working directory to its Claude Code transcript
// directory under the config dir.
package project

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	configDirEnv = "CLAUDE_CONFIG_DIR"
	projectsSub  = "projects"
	maxEncLen    = 200
)

var nonAlnum = regexp.MustCompile(`[^a-zA-Z0-9]`)

// ConfigDir returns the Claude Code config directory, honoring CLAUDE_CONFIG_DIR
// the same way Claude Code itself does.
func ConfigDir() (string, error) {
	if dir := os.Getenv(configDirEnv); dir != "" {
		if !filepath.IsAbs(dir) {
			return "", fmt.Errorf("%s must be an absolute path: %s", configDirEnv, dir)
		}
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".claude"), nil
}

// EncodedName converts a working directory to Claude Code's directory name.
// Every non-alphanumeric byte becomes '-'; names over 200 bytes are truncated
// and suffixed with a base36 hash of the full path.
func EncodedName(cwd string) string {
	enc := nonAlnum.ReplaceAllString(cwd, "-")
	if len(enc) <= maxEncLen {
		return enc
	}
	return enc[:maxEncLen] + "-" + strconv.FormatInt(absInt64(pathHash(cwd)), 36)
}

// pathHash mirrors Claude Code's Java-style string hash: h = h*31 + rune,
// wrapping in int32.
func pathHash(s string) int32 {
	var h int32
	for _, r := range s {
		h = h*31 + int32(r)
	}
	return h
}

func absInt64(v int32) int64 {
	if v < 0 {
		return -int64(v)
	}
	return int64(v)
}

// Resolve returns the project transcript directory for the current working
// directory. Symlinks are resolved first because Claude Code stores the
// physical path.
func Resolve() (dir string, encoded string, err error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", fmt.Errorf("failed to get working directory: %w", err)
	}
	real, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve %s: %w", cwd, err)
	}
	cfg, err := ConfigDir()
	if err != nil {
		return "", "", err
	}
	enc := EncodedName(real)
	return filepath.Join(cfg, projectsSub, enc), enc, nil
}

// CwdFromEncoded is the inverse hint used only for display. The encoding is
// lossy, so this returns the best-effort path with '-' separators.
func CwdFromEncoded(enc string) string {
	return strings.ReplaceAll(enc, "-", "/")
}
