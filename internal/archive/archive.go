// Package archive packs a project directory into a compressed tar stream.
package archive

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
)

// Extension is the file suffix used for archives.
const Extension = "tar.zst"

// Pack writes the directory rootDir (stored under the base name of rootDir)
// to w as a zstd-compressed tar stream.
func Pack(w io.Writer, rootDir string) error {
	enc, err := zstd.NewWriter(w, zstd.WithEncoderLevel(zstd.SpeedBetterCompression))
	if err != nil {
		return err
	}
	tw := tar.NewWriter(enc)

	err = filepath.Walk(rootDir, func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(filepath.Dir(rootDir), p)
		if err != nil {
			return err
		}
		if info.IsDir() {
			return tw.WriteHeader(&tar.Header{
				Typeflag: tar.TypeDir,
				Name:     rel + "/",
				Mode:     0o755,
				ModTime:  info.ModTime(),
			})
		}

		hdr := &tar.Header{
			Typeflag: tar.TypeReg,
			Name:     rel,
			Mode:     0o600,
			Size:     info.Size(),
			ModTime:  info.ModTime(),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.CopyN(tw, f, info.Size())
		return err
	})
	if err != nil {
		return err
	}

	if err := tw.Close(); err != nil {
		return err
	}
	return enc.Close()
}

// Unpack extracts a zstd-compressed tar stream into destRoot. Every entry is
// written under destRoot; absolute paths and ".." components are rejected.
func Unpack(r io.Reader, destRoot string) error {
	dec, err := zstd.NewReader(r)
	if err != nil {
		return err
	}
	defer dec.Close()

	tr := tar.NewReader(dec)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("corrupt archive: %w", err)
		}

		target, err := safeJoin(destRoot, hdr.Name)
		if err != nil {
			return err
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := writeFile(target, tr, hdr.Size); err != nil {
				return err
			}
		default:
			return fmt.Errorf("corrupt archive: unexpected entry type %d for %s", hdr.Typeflag, hdr.Name)
		}
	}
	return nil
}

func writeFile(target string, r io.Reader, size int64) error {
	f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.CopyN(f, r, size); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func safeJoin(destRoot, name string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(name))
	if filepath.IsAbs(clean) {
		return "", fmt.Errorf("corrupt archive: absolute path %q", name)
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("corrupt archive: path escapes destination: %q", name)
	}
	return filepath.Join(destRoot, clean), nil
}
