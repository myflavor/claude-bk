package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEncodedName(t *testing.T) {
	longPath := "/tmp/aa/" +
		repeat("a", 40) + "/" + repeat("b", 40) + "/" +
		repeat("c", 40) + "/" + repeat("d", 40) + "/" + repeat("e", 30)

	cases := []struct {
		name string
		cwd  string
		want string
	}{
		{"simple", "/home/xin/mg4", "-home-xin-mg4"},
		{"nested", "/mnt/d/Work/Projects/bangbang/bangbang-all", "-mnt-d-Work-Projects-bangbang-bangbang-all"},
		{"underscore", "/mnt/c/Users/Xin/Desktop/sess_375ecf45-export", "-mnt-c-Users-Xin-Desktop-sess-375ecf45-export"},
		{"non-ascii", "/mnt/c/Users/Xin/Desktop/新建文件夹", "-mnt-c-Users-Xin-Desktop------"},
		{"trailing name", "/home/xin", "-home-xin"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := EncodedName(tc.cwd); got != tc.want {
				t.Fatalf("EncodedName(%q)\n got: %s\nwant: %s", tc.cwd, got, tc.want)
			}
		})
	}

	// Verified against a real Claude Code build: it accepts a directory named
	// with the 200-byte prefix and the base36 hash suffix.
	got := EncodedName(longPath)
	const suffix = "-kgrvhk"
	if !strings.HasSuffix(got, suffix) {
		t.Fatalf("long path suffix: got %s, want ...%s", got, suffix)
	}
	if prefixLen := len(got) - len(suffix); prefixLen != maxEncLen {
		t.Fatalf("long path prefix length: got %d, want %d", prefixLen, maxEncLen)
	}
	if want := nonAlnum.ReplaceAllString(longPath, "-")[:maxEncLen]; got[:maxEncLen] != want {
		t.Fatalf("long path prefix mismatch:\n got: %s\nwant: %s", got[:maxEncLen], want)
	}
}

func repeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}

func TestConfigDirHonoursEnv(t *testing.T) {
	t.Setenv(configDirEnv, "/tmp/fake-claude-home")
	got, err := ConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != "/tmp/fake-claude-home" {
		t.Fatalf("got %s", got)
	}
}

func TestConfigDirRejectsRelative(t *testing.T) {
	t.Setenv(configDirEnv, "relative/path")
	if _, err := ConfigDir(); err == nil {
		t.Fatal("expected error for relative CLAUDE_CONFIG_DIR")
	}
}

func TestResolveFollowsSymlinks(t *testing.T) {
	real := t.TempDir()
	linkParent := t.TempDir()
	link := filepath.Join(linkParent, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skip("symlinks unavailable:", err)
	}

	t.Setenv(configDirEnv, "/tmp/cfg")
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(link); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(wd) })

	_, enc, err := Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if want := EncodedName(real); enc != want {
		t.Fatalf("got %s, want %s (resolved target of symlink)", enc, want)
	}
}
