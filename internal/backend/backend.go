// Package backend abstracts where archives are stored.
package backend

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"claude-bk/internal/provider"
)

// Store is a flat object store keyed by file name.
type Store interface {
	// Put writes name from r, replacing any existing object.
	Put(name string, r io.Reader) error
	// Get opens name for reading.
	Get(name string) (io.ReadCloser, error)
	// Close releases the underlying connection.
	Close() error
}

// Open builds a Store from a provider config.
func Open(cfg *provider.Config) (Store, error) {
	switch cfg.Type {
	case "local":
		return &localStore{dir: cfg.Path}, nil
	case "smb":
		return newSMBStore(cfg)
	default:
		return nil, fmt.Errorf("unknown type %q", cfg.Type)
	}
}

type localStore struct{ dir string }

func (s *localStore) Put(name string, r io.Reader) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.dir, ".upload-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, filepath.Join(s.dir, name))
}

func (s *localStore) Get(name string) (io.ReadCloser, error) {
	f, err := os.Open(filepath.Join(s.dir, name))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no backup found for this project: %s", name)
		}
		return nil, err
	}
	return f, nil
}

func (s *localStore) Close() error { return nil }
