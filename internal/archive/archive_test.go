package archive

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"
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
	n, err := Pack(&buf, src)
	if err != nil {
		t.Fatal(err)
	}
	var wantBytes int64
	for _, c := range files {
		wantBytes += int64(len(c))
	}
	if n != wantBytes {
		t.Fatalf("Pack reported %d input bytes, want %d", n, wantBytes)
	}

	dest := t.TempDir()
	if err := Unpack(bytes.NewReader(buf.Bytes()), dest); err != nil {
		t.Fatal(err)
	}

	for name, want := range files {
		got, err := os.ReadFile(filepath.Join(dest, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if string(got) != want {
			t.Fatalf("%s: got %q, want %q", name, got, want)
		}
	}
}

func TestPackUnpackAreInverses(t *testing.T) {
	src := filepath.Join(t.TempDir(), "-mnt-d-Projects-coolapk")
	writeTree(t, src, map[string]string{"s.jsonl": "x"})

	var buf bytes.Buffer
	if _, err := Pack(&buf, src); err != nil {
		t.Fatal(err)
	}

	dest := t.TempDir()
	if err := Unpack(&buf, dest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "s.jsonl")); err != nil {
		t.Fatalf("content not restored at the destination root: %v", err)
	}
}

func TestPackOmitsRootEntry(t *testing.T) {
	src := filepath.Join(t.TempDir(), "-home-xin-mg4")
	writeTree(t, src, map[string]string{"a.jsonl": "x", "memory/MEMORY.md": "y"})

	var buf bytes.Buffer
	if _, err := Pack(&buf, src); err != nil {
		t.Fatal(err)
	}

	dec, err := zstd.NewReader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	defer dec.Close()

	tr := tar.NewReader(dec)
	var names []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, hdr.Name)
	}

	for _, name := range names {
		if name == "." || name == "./" {
			t.Fatalf("archive contains a root entry %q; entries: %v", name, names)
		}
	}
	want := map[string]bool{"a.jsonl": false, "memory": false, "memory/MEMORY.md": false}
	for _, name := range names {
		if _, ok := want[name]; !ok {
			t.Fatalf("unexpected entry %q; entries: %v", name, names)
		}
		want[name] = true
	}
	for name, seen := range want {
		if !seen {
			t.Fatalf("missing entry %q; entries: %v", name, names)
		}
	}
}

func TestUnpackRejectsPathTraversal(t *testing.T) {
	for _, name := range []string{"../escape", "a/../../escape", "/abs/path"} {
		t.Run(name, func(t *testing.T) {
			dest := t.TempDir()
			outside := filepath.Join(filepath.Dir(dest), "escape")
			defer os.RemoveAll(outside)

			if err := Unpack(maliciousArchive(t, name), dest); err == nil {
				t.Fatalf("expected rejection for %q", name)
			}
			if _, err := os.Stat(outside); err == nil {
				t.Fatalf("%q escaped the destination", name)
			}
		})
	}
}

func maliciousArchive(t *testing.T, name string) *bytes.Reader {
	t.Helper()
	var buf bytes.Buffer
	enc, err := zstd.NewWriter(&buf)
	if err != nil {
		t.Fatal(err)
	}
	tw := tar.NewWriter(enc)
	body := []byte("pwned")
	if err := tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeReg,
		Name:     name,
		Mode:     0o600,
		Size:     int64(len(body)),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := enc.Close(); err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(buf.Bytes())
}

func TestUnpackRejectsCorruptStream(t *testing.T) {
	if err := Unpack(bytes.NewReader([]byte("not a zstd stream")), t.TempDir()); err == nil {
		t.Fatal("expected error for corrupt input")
	}
}

func TestUnpackRejectsUnexpectedEntryType(t *testing.T) {
	var buf bytes.Buffer
	enc, _ := zstd.NewWriter(&buf)
	tw := tar.NewWriter(enc)
	if err := tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeSymlink,
		Name:     "link",
		Linkname: "/etc/passwd",
	}); err != nil {
		t.Fatal(err)
	}
	tw.Close()
	enc.Close()

	if err := Unpack(&buf, t.TempDir()); err == nil {
		t.Fatal("expected error for symlink entry")
	}
}
