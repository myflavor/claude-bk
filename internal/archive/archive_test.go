package archive

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRoundTrip(t *testing.T) {
	src := filepath.Join(t.TempDir(), "-home-xin-mg4")
	files := map[string]string{
		"abc.jsonl":                      `{"type":"user"}` + "\n",
		"abc/subagents/agent-1.jsonl":    "sub",
		"abc/tool-results/out.txt":       "big output",
		"memory/MEMORY.md":               "# Memory index\n",
		"memory/user-prefers-chinese.md": "- 中文",
	}
	writeTree(t, src, files)

	var buf bytes.Buffer
	if err := Pack(&buf, src); err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Fatal("empty archive")
	}

	dest := t.TempDir()
	if err := Unpack(bytes.NewReader(buf.Bytes()), dest); err != nil {
		t.Fatal(err)
	}

	for name, want := range files {
		got, err := os.ReadFile(filepath.Join(dest, "-home-xin-mg4", name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if string(got) != want {
			t.Fatalf("%s: got %q, want %q", name, got, want)
		}
	}
}

func TestRoundTripPreservesTopLevelDir(t *testing.T) {
	parent := t.TempDir()
	src := filepath.Join(parent, "-mnt-d-Projects-coolapk")
	writeTree(t, src, map[string]string{"s.jsonl": "x"})

	var buf bytes.Buffer
	if err := Pack(&buf, src); err != nil {
		t.Fatal(err)
	}

	dest := t.TempDir()
	if err := Unpack(&buf, dest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "-mnt-d-Projects-coolapk", "s.jsonl")); err != nil {
		t.Fatalf("top-level directory not preserved: %v", err)
	}
}

func TestUnpackRejectsPathTraversal(t *testing.T) {
	for _, name := range []string{"../escape", "a/../../escape"} {
		if _, err := safeJoin("/tmp/dest", name); err == nil {
			t.Fatalf("expected rejection for %q", name)
		}
	}
	if _, err := safeJoin("/tmp/dest", "/abs/path"); err == nil {
		t.Fatal("expected rejection for absolute path")
	}
}

func TestUnpackRejectsCorruptStream(t *testing.T) {
	err := Unpack(bytes.NewReader([]byte("not a zstd stream")), t.TempDir())
	if err == nil {
		t.Fatal("expected error for corrupt input")
	}
}
