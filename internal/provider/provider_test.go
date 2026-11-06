package provider

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, home, name, body string) {
	t.Helper()
	dir := filepath.Join(home, configDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func withHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

func TestLoadSMBDefaults(t *testing.T) {
	home := withHome(t)
	write(t, home, "nas", `{"type":"smb","host":"10.0.0.1","share":"backup","user":"u","password":"p"}`)

	cfg, err := Load("nas")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 445 {
		t.Fatalf("port default: got %d", cfg.Port)
	}
}

func TestLoadLocal(t *testing.T) {
	home := withHome(t)
	write(t, home, "loc", `{"type":"local","path":"/tmp/store"}`)

	cfg, err := Load("loc")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Path != "/tmp/store" {
		t.Fatalf("got %q", cfg.Path)
	}
}

func TestLoadRejectsMissingFields(t *testing.T) {
	home := withHome(t)
	cases := map[string]string{
		"no password": `{"type":"smb","host":"h","share":"s","user":"u"}`,
		"no share":    `{"type":"smb","host":"h","user":"u","password":"p"}`,
		"no path":     `{"type":"local"}`,
		"relative":    `{"type":"local","path":"rel/path"}`,
		"bad type":    `{"type":"ftp","host":"h"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			write(t, home, "p", body)
			if _, err := Load("p"); err == nil {
				t.Fatalf("expected error for %s", name)
			}
		})
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	home := withHome(t)
	write(t, home, "bad", `{not json}`)
	if _, err := Load("bad"); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadNotFoundListsAvailable(t *testing.T) {
	home := withHome(t)
	write(t, home, "alpha", `{"type":"local","path":"/tmp/a"}`)
	write(t, home, "beta", `{"type":"local","path":"/tmp/b"}`)

	_, err := Load("missing")
	if err == nil {
		t.Fatal("expected error")
	}
	for _, want := range []string{"alpha", "beta"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error should list %q: %v", want, err)
		}
	}
}
