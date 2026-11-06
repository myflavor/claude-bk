package backend

import (
	"fmt"
	"io"
	"net"
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

	dialer := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     cfg.User,
			Password: cfg.Password,
		},
	}
	session, err := dialer.Dial(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("SMB authentication failed for %s: %w", addr, err)
	}

	share, err := session.Mount(cfg.Share)
	if err != nil {
		session.Logoff()
		conn.Close()
		return nil, fmt.Errorf("failed to mount share %q on %s: %w", cfg.Share, addr, err)
	}

	root, err := normaliseRoot(cfg.Root)
	if err != nil {
		share.Umount()
		session.Logoff()
		conn.Close()
		return nil, err
	}
	if root != "" {
		if err := share.MkdirAll(root, 0o755); err != nil {
			share.Umount()
			session.Logoff()
			conn.Close()
			return nil, fmt.Errorf("failed to create root directory %q on share %q: %w", root, cfg.Share, err)
		}
	}

	return &smbStore{conn: conn, session: session, share: share, root: root}, nil
}

// normaliseRoot turns a configured root into a share-relative SMB path.
func normaliseRoot(root string) (string, error) {
	root = strings.TrimSpace(root)
	if root == "" || root == "." {
		return "", nil
	}
	if filepath.IsAbs(root) {
		return "", fmt.Errorf("root must be relative to the share, got absolute path %q", root)
	}
	root = path.Clean(strings.ReplaceAll(root, `\`, "/"))
	if root == "." {
		return "", nil
	}
	if strings.HasPrefix(root, "..") {
		return "", fmt.Errorf("root must not escape the share: %q", root)
	}
	return root, nil
}

func (s *smbStore) objectPath(name string) string {
	if s.root == "" {
		return name
	}
	return s.root + "/" + name
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

	if err := s.share.Remove(target); err != nil && !isNotExist(err) {
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
		if isNotExist(err) {
			return nil, fmt.Errorf("no backup found for this project: %s", name)
		}
		return nil, fmt.Errorf("failed to open %s: %w", name, err)
	}
	return f, nil
}

func (s *smbStore) Close() error {
	var firstErr error
	if s.share != nil {
		if err := s.share.Umount(); err != nil {
			firstErr = err
		}
	}
	if s.session != nil {
		if err := s.session.Logoff(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if s.conn != nil {
		if err := s.conn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func isNotExist(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "does not exist") ||
		strings.Contains(msg, "not exist") ||
		strings.Contains(msg, "not found") ||
		strings.Contains(msg, "STATUS_OBJECT_NAME_NOT_FOUND") ||
		strings.Contains(msg, "STATUS_OBJECT_PATH_NOT_FOUND")
}
