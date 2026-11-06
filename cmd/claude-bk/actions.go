package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"claude-bk/internal/archive"
	"claude-bk/internal/backend"
	"claude-bk/internal/project"
	"claude-bk/internal/provider"
)

func runPush(providerName string) error {
	dir, enc, store, closeStore, err := prepare(providerName)
	if err != nil {
		return err
	}
	defer closeStore()
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return fmt.Errorf("no conversations for this directory (expected %s)", dir)
	}

	name := enc + "." + archive.Extension
	fmt.Printf("Packing %s\n", dir)

	size := &countingWriter{}
	pr, pw := io.Pipe()
	packErr := make(chan error, 1)
	go func() {
		// io.Pipe is unbuffered, so without this the zstd encoder's ~24 KB
		// blocks reach the SMB writer as individual requests. Buffering to
		// the library's 1 MB request cap turns ~1000 round trips into ~25.
		bw := bufio.NewWriterSize(pw, 1<<20)
		_, err := archive.Pack(io.MultiWriter(bw, size), dir)
		if err == nil {
			err = bw.Flush()
		}
		pw.CloseWithError(err)
		packErr <- err
	}()

	fmt.Printf("Uploading %s\n", name)
	if err := store.Put(name, pr); err != nil {
		pr.CloseWithError(err)
		<-packErr
		return err
	}
	pr.Close()
	if err := <-packErr; err != nil {
		return err
	}

	fmt.Printf("Done (%s).\n", humanBytes(size.n))
	return nil
}

func runPull(providerName string) error {
	dir, enc, store, closeStore, err := prepare(providerName)
	if err != nil {
		return err
	}
	defer closeStore()

	name := enc + "." + archive.Extension
	rc, err := store.Get(name)
	if err != nil {
		return err
	}
	defer rc.Close()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	fmt.Printf("Restoring %s\n", dir)
	// The zstd decoder reads its input in small unbuffered pieces (block
	// headers are 1–4 bytes), which would become one SMB request each.
	if err := archive.Unpack(bufio.NewReaderSize(rc, 1<<20), dir); err != nil {
		return fmt.Errorf("failed to unpack %s: %w", name, err)
	}
	fmt.Println("Done.")
	return nil
}

// prepare resolves the current project and opens the provider's store. The
// returned close function must be called by the caller.
func prepare(providerName string) (dir, enc string, store backend.Store, closeStore func(), err error) {
	cfg, err := provider.Load(providerName)
	if err != nil {
		return "", "", nil, nil, err
	}
	dir, enc, err = project.Resolve()
	if err != nil {
		return "", "", nil, nil, err
	}
	store, err = backend.Open(cfg)
	if err != nil {
		return "", "", nil, nil, err
	}
	return dir, enc, store, func() { store.Close() }, nil
}

type countingWriter struct{ n int64 }

func (c *countingWriter) Write(p []byte) (int, error) {
	c.n += int64(len(p))
	return len(p), nil
}
