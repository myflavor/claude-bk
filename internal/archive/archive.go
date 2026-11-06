// Package archive packs a project directory into a compressed tar stream.
package archive

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/klauspost/compress/zstd"
)

// Extension is the file suffix used for archives.
const Extension = "tar.zst"

// Pack writes dir to w as a zstd-compressed tar stream. Entries are stored
// relative to dir, so Unpack reproduces its contents. It returns the number of
// file-content bytes packed.
func Pack(w io.Writer, dir string) (int64, error) {
	enc, err := zstd.NewWriter(w, zstd.WithEncoderLevel(zstd.SpeedBetterCompression))
	if err != nil {
		return 0, err
	}
	tw := tar.NewWriter(enc)

	var inputBytes int64
	err = filepath.Walk(dir, func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		if err := writeEntry(tw, p, rel, info); err != nil {
			return err
		}
		if !info.IsDir() {
			inputBytes += info.Size()
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	if err := tw.Close(); err != nil {
		return 0, err
	}
	return inputBytes, enc.Close()
}

func writeEntry(tw *tar.Writer, path, name string, info os.FileInfo) error {
	hdr, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	hdr.Name = name
	hdr.Mode = 0o600
	if info.IsDir() {
		hdr.Mode = 0o755
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if info.IsDir() {
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.CopyN(tw, f, info.Size())
	return err
}

// Unpack extracts a zstd-compressed tar stream into dir, which must exist.
// Every entry is confined to dir; escaping names are rejected by the OS.
func Unpack(r io.Reader, dir string) error {
	dec, err := zstd.NewReader(r)
	if err != nil {
		return err
	}
	defer dec.Close()

	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()

	tr := tar.NewReader(dec)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("corrupt archive: %w", err)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := root.MkdirAll(hdr.Name, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := root.MkdirAll(filepath.Dir(hdr.Name), 0o755); err != nil {
				return err
			}
			if err := writeFile(root, hdr.Name, tr, hdr.Size); err != nil {
				return err
			}
		default:
			return fmt.Errorf("corrupt archive: unexpected entry type %d for %s", hdr.Typeflag, hdr.Name)
		}
	}
}

func writeFile(root *os.Root, name string, r io.Reader, size int64) error {
	f, err := root.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.CopyN(f, r, size); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
