package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/oauth2"
)

// tokenFileMode is the permission the token cache is written with. The
// file holds a bearer credential, so it stays readable by its owner only.
const tokenFileMode = 0o600

type cachingTokenSource struct {
	filePath string
	new      oauth2.TokenSource

	mu sync.Mutex
}

// NewCachingTokenSource returns a TokenSource that caches the token from the
// provided TokenSource. It is based on the implementation of
// oauth2.ReuseTokenSource.
func NewCachingTokenSource(filePath string, src oauth2.TokenSource) oauth2.TokenSource {
	return &cachingTokenSource{
		filePath: filePath,
		new:      src,
	}
}

func (s *cachingTokenSource) Token() (*oauth2.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cached, err := loadTokenFromDisk(s.filePath)
	switch {
	case err == nil && cached.Valid():
		return cached, nil
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		// A first run has no cache; anything else is worth reporting
		// before it is silently replaced.
		slog.Warn("ignoring an unreadable OAuth token cache", "path", s.filePath, "error", err)
	}

	t, err := s.new.Token()
	if err != nil {
		return nil, err
	}

	// A token that cannot be cached is still a good token: returning the
	// error here would turn a successful login into a failed command.
	if err := saveTokenToDisk(s.filePath, t); err != nil {
		slog.Warn("the OAuth token could not be cached; the next command will ask you to log in again",
			"path", s.filePath, "error", err)
	}

	return t, nil
}

func loadTokenFromDisk(path string) (*oauth2.Token, error) {
	// os.ReadFile rather than os.Open + Decode so callers can tell a
	// missing cache from a corrupt one with errors.Is(err, fs.ErrNotExist).
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var t oauth2.Token
	if err := json.Unmarshal(body, &t); err != nil {
		return nil, fmt.Errorf("failed to parse the OAuth token cache %s: %w", path, err)
	}
	return &t, nil
}

// saveTokenToDisk writes the token to p, replacing any existing file. The
// write goes to a temporary file first and is then renamed into place, so
// a second process never observes a half-written cache.
func saveTokenToDisk(p string, t *oauth2.Token) error {
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create credential cache directory, %w", err)
	}

	body, err := json.Marshal(t)
	if err != nil {
		return err
	}
	body = append(body, '\n')

	f, err := os.CreateTemp(dir, ".geni-token-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	// A no-op once the rename below succeeds.
	defer func() { _ = os.Remove(tmp) }()

	if err := writeTokenFile(f, body); err != nil {
		return err
	}

	return os.Rename(tmp, p)
}

// writeTokenFile writes and flushes the token, closing f either way.
func writeTokenFile(f *os.File, body []byte) error {
	defer func() { _ = f.Close() }()

	// CreateTemp already made the file private, but say so explicitly:
	// the mode matters more than the default.
	if err := f.Chmod(tokenFileMode); err != nil {
		return err
	}
	if _, err := f.Write(body); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	return f.Close()
}
