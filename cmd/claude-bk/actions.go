package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"claude-bk/internal/archive"
	"claude-bk/internal/backend"
	"claude-bk/internal/project"
	"claude-bk/internal/provider"
)

func runPush(providerName string) error {
	cfg, err := provider.Load(providerName)
	if err != nil {
		return err
	}

	dir, enc, err := project.Resolve()
	if err != nil {
		return err
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return fmt.Errorf("no conversations for this directory (expected %s)", dir)
	}

	store, err := backend.Open(cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	name := enc + "." + archive.Extension
	fmt.Printf("Packing %s (%s)\n", dir, humanBytes(dirSize(dir)))

	counter := &countingReader{}
	pr, pw := io.Pipe()
	packErr := make(chan error, 1)
	go func() {
		err := archive.Pack(io.MultiWriter(pw, counter), dir)
		pw.CloseWithError(err)
		packErr <- err
	}()

	fmt.Printf("Uploading %s\n", name)
	putErr := store.Put(name, pr)
	pr.Close()

	if putErr != nil {
		<-packErr
		return putErr
	}
	if err := <-packErr; err != nil {
		return err
	}

	fmt.Printf("Done (%s).\n", humanBytes(counter.n))
	return nil
}

func runPull(providerName string) error {
	cfg, err := provider.Load(providerName)
	if err != nil {
		return err
	}

	dir, enc, err := project.Resolve()
	if err != nil {
		return err
	}

	store, err := backend.Open(cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	name := enc + "." + archive.Extension
	rc, err := store.Get(name)
	if err != nil {
		return err
	}
	defer rc.Close()

	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return err
	}

	fmt.Printf("Restoring %s\n", dir)
	if err := archive.Unpack(rc, filepath.Dir(dir)); err != nil {
		return fmt.Errorf("failed to unpack %s: %w", name, err)
	}
	fmt.Println("Done.")
	return nil
}

type countingReader struct{ n int64 }

func (c *countingReader) Write(p []byte) (int, error) {
	c.n += int64(len(p))
	return len(p), nil
}
