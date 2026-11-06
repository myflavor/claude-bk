package backend

import (
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/hirochachacha/go-smb2"

	"claude-bk/internal/provider"
)

type smbStore struct {
	conn    net.Conn
	session *smb2.Session
	share   *smb2.Share
	root    string
}

func newSMBStore(cfg *provider.Config) (Store, error) {
	addr := net.JoinHostPort(cfg.Host, fmt.Sprint(cfg.Port))
	conn, err := net.DialTimeout("tcp", addr, 15*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	s := &smbStore{conn: conn}
	dialer := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     cfg.User,
			Password: cfg.Password,
		},
	}
	if s.session, err = dialer.Dial(conn); err != nil {
		s.Close()
		return nil, fmt.Errorf("SMB authentication failed for %s: %w", addr, err)
	}
	if s.share, err = s.session.Mount(cfg.Share); err != nil {
		s.Close()
		return nil, fmt.Errorf("failed to mount share %q on %s: %w", cfg.Share, addr, err)
	}

	root, err := normaliseRoot(cfg.Root)
	if err != nil {
		s.Close()
		return nil, err
	}
	s.root = root
	if root != "" {
		if err := s.share.MkdirAll(root, 0o755); err != nil {
			s.Close()
			return nil, fmt.Errorf("failed to create root directory %q on share %q: %w", root, cfg.Share, err)
		}
	}
	return s, nil
}

// normaliseRoot turns a configured root into a share-relative SMB path.
func normaliseRoot(root string) (string, error) {
	root = path.Clean(filepath.ToSlash(strings.TrimSpace(root)))
	if root == "." || root == "/" {
		return "", nil
	}
	if !filepath.IsLocal(root) {
		return "", fmt.Errorf("root must be a relative path inside the share: %q", root)
	}
	return root, nil
}

func (s *smbStore) objectPath(name string) string {
	return path.Join(s.root, name)
}

func (s *smbStore) Put(name string, r io.Reader) error {
	target := s.objectPath(name)
	tmp := target + ".part"

	f, err := s.share.Create(tmp)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", tmp, err)
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		s.share.Remove(tmp)
		return fmt.Errorf("failed to upload %s: %w", name, err)
	}
	if err := f.Close(); err != nil {
		s.share.Remove(tmp)
		return fmt.Errorf("failed to close %s: %w", tmp, err)
	}

	if err := s.share.Remove(target); err != nil && !os.IsNotExist(err) {
		s.share.Remove(tmp)
		return fmt.Errorf("failed to replace existing %s: %w", name, err)
	}
	if err := s.share.Rename(tmp, target); err != nil {
		s.share.Remove(tmp)
		return fmt.Errorf("failed to finalise %s: %w", name, err)
	}
	return nil
}

func (s *smbStore) Get(name string) (io.ReadCloser, error) {
	f, err := s.share.Open(s.objectPath(name))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errNotFound(name)
		}
		return nil, fmt.Errorf("failed to open %s: %w", name, err)
	}
	return f, nil
}

func (s *smbStore) Close() error {
	if s.share != nil {
		s.share.Umount()
	}
	if s.session != nil {
		s.session.Logoff()
	}
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}
