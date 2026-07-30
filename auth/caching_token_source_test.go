package auth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"golang.org/x/oauth2"
)

// countingTokenSource records how often the interactive flow was reached.
type countingTokenSource struct {
	token *oauth2.Token
	err   error
	calls int
}

func (s *countingTokenSource) Token() (*oauth2.Token, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return s.token, nil
}

func freshToken(accessToken string) *oauth2.Token {
	return &oauth2.Token{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		Expiry:      time.Now().Add(time.Hour),
	}
}

// writeCache seeds the cache file with the given JSON body.
func writeCache(t *testing.T, path, body string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("failed to seed the token cache: %v", err)
	}
}

func TestCachingTokenSourceToken(t *testing.T) {
	t.Run("Returns the cached token without a new login", func(t *testing.T) {
		RegisterTestingT(t)

		path := filepath.Join(t.TempDir(), "token.json")
		Expect(saveTokenToDisk(path, freshToken("cached"))).To(Succeed())

		inner := &countingTokenSource{token: freshToken("fresh")}
		token, err := NewCachingTokenSource(path, inner).Token()

		Expect(err).ToNot(HaveOccurred())
		Expect(token.AccessToken).To(Equal("cached"))
		Expect(inner.calls).To(Equal(0))
	})

	t.Run("Logs in again when the cached token has expired", func(t *testing.T) {
		RegisterTestingT(t)

		path := filepath.Join(t.TempDir(), "token.json")
		expired := &oauth2.Token{AccessToken: "stale", Expiry: time.Now().Add(-time.Hour)}
		Expect(saveTokenToDisk(path, expired)).To(Succeed())

		inner := &countingTokenSource{token: freshToken("fresh")}
		token, err := NewCachingTokenSource(path, inner).Token()

		Expect(err).ToNot(HaveOccurred())
		Expect(token.AccessToken).To(Equal("fresh"))
		Expect(inner.calls).To(Equal(1))

		stored, err := loadTokenFromDisk(path)
		Expect(err).ToNot(HaveOccurred())
		Expect(stored.AccessToken).To(Equal("fresh"))
	})

	// A cache that cannot be parsed is replaced, but not in silence: it
	// usually means something else wrote the file.
	t.Run("Warns and logs in again when the cache is corrupt", func(t *testing.T) {
		RegisterTestingT(t)

		path := filepath.Join(t.TempDir(), "token.json")
		writeCache(t, path, "not json")

		inner := &countingTokenSource{token: freshToken("fresh")}

		var token *oauth2.Token
		var err error
		logs := captureSlog(t, func() { token, err = NewCachingTokenSource(path, inner).Token() })

		Expect(err).ToNot(HaveOccurred())
		Expect(token.AccessToken).To(Equal("fresh"))
		Expect(inner.calls).To(Equal(1))
		Expect(logs).To(ContainSubstring("unreadable OAuth token cache"))
	})

	// A first run has no cache. That is not an anomaly and must not look
	// like one.
	t.Run("Says nothing when there is no cache yet", func(t *testing.T) {
		RegisterTestingT(t)

		path := filepath.Join(t.TempDir(), "token.json")
		inner := &countingTokenSource{token: freshToken("fresh")}

		var err error
		logs := captureSlog(t, func() { _, err = NewCachingTokenSource(path, inner).Token() })

		Expect(err).ToNot(HaveOccurred())
		Expect(logs).To(BeEmpty())
	})

	t.Run("Returns the login error and writes nothing", func(t *testing.T) {
		RegisterTestingT(t)

		path := filepath.Join(t.TempDir(), "token.json")
		inner := &countingTokenSource{err: errors.New("login failed")}

		_, err := NewCachingTokenSource(path, inner).Token()

		Expect(err).To(MatchError(ContainSubstring("login failed")))
		Expect(path).ToNot(BeAnExistingFile())
	})

	// Losing the cache is an inconvenience, not a failed login: the token
	// in hand is valid and must reach the caller.
	t.Run("Returns the token even when it cannot be cached", func(t *testing.T) {
		if runtime.GOOS == "windows" || os.Geteuid() == 0 {
			t.Skip("directory permissions do not block writes here")
		}
		RegisterTestingT(t)

		dir := filepath.Join(t.TempDir(), "readonly")
		Expect(os.Mkdir(dir, 0o500)).To(Succeed())
		t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

		inner := &countingTokenSource{token: freshToken("fresh")}

		var token *oauth2.Token
		var err error
		logs := captureSlog(t, func() {
			token, err = NewCachingTokenSource(filepath.Join(dir, "token.json"), inner).Token()
		})

		Expect(err).ToNot(HaveOccurred())
		Expect(token.AccessToken).To(Equal("fresh"))
		Expect(logs).To(ContainSubstring("could not be cached"))
	})
}

// fakeRefresher stands in for the token endpoint.
type fakeRefresher struct {
	token *oauth2.Token
	err   error
	seen  []string
}

func (r *fakeRefresher) Refresh(_ context.Context, refreshToken string) (*oauth2.Token, error) {
	r.seen = append(r.seen, refreshToken)
	if r.err != nil {
		return nil, r.err
	}
	return r.token, nil
}

// storedRefreshToken is the refresh token seedExpired plants in the
// cache, and so the one a refresh is expected to present.
const storedRefreshToken = "old-rt"

// seedExpired writes an expired token carrying a refresh token and
// returns the cache path.
func seedExpired(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "token.json")
	expired := &oauth2.Token{
		AccessToken:  "stale",
		RefreshToken: storedRefreshToken,
		Expiry:       time.Now().Add(-time.Hour),
	}
	if err := saveTokenToDisk(path, expired); err != nil {
		t.Fatalf("failed to seed the token cache: %v", err)
	}
	return path
}

func TestRefreshingCachingTokenSource(t *testing.T) {
	// The whole point: renewing has to reach the disk, because Geni
	// rotates the refresh token on every refresh.
	t.Run("Persists the refreshed token instead of logging in again", func(t *testing.T) {
		RegisterTestingT(t)

		path := seedExpired(t)
		refresher := &fakeRefresher{token: &oauth2.Token{
			AccessToken:  "new-at",
			RefreshToken: "new-rt",
			Expiry:       time.Now().Add(time.Hour),
		}}
		inner := &countingTokenSource{token: freshToken("interactive")}

		token, err := NewRefreshingCachingTokenSource(path, refresher, inner).Token()

		Expect(err).ToNot(HaveOccurred())
		Expect(token.AccessToken).To(Equal("new-at"))
		Expect(inner.calls).To(Equal(0))
		Expect(refresher.seen).To(Equal([]string{"old-rt"}))

		stored, err := loadTokenFromDisk(path)
		Expect(err).ToNot(HaveOccurred())
		Expect(stored.AccessToken).To(Equal("new-at"))
		Expect(stored.RefreshToken).To(Equal("new-rt"))
	})

	t.Run("Keeps the old refresh token when the response omits one", func(t *testing.T) {
		RegisterTestingT(t)

		path := seedExpired(t)
		refresher := &fakeRefresher{token: &oauth2.Token{
			AccessToken: "new-at",
			Expiry:      time.Now().Add(time.Hour),
		}}

		_, err := NewRefreshingCachingTokenSource(path, refresher, &countingTokenSource{}).Token()

		Expect(err).ToNot(HaveOccurred())
		stored, err := loadTokenFromDisk(path)
		Expect(err).ToNot(HaveOccurred())
		Expect(stored.RefreshToken).To(Equal("old-rt"))
	})

	t.Run("Logs in again when the refresh token is rejected", func(t *testing.T) {
		RegisterTestingT(t)

		path := seedExpired(t)
		refresher := &fakeRefresher{err: fmt.Errorf("%w: Invalid refresh token", ErrRefreshRejected)}
		inner := &countingTokenSource{token: freshToken("interactive")}

		token, err := NewRefreshingCachingTokenSource(path, refresher, inner).Token()

		Expect(err).ToNot(HaveOccurred())
		Expect(token.AccessToken).To(Equal("interactive"))
		Expect(inner.calls).To(Equal(1))
	})

	// Geni being down is not a reason to open a browser.
	t.Run("Reports a transient refresh failure without logging in", func(t *testing.T) {
		RegisterTestingT(t)

		path := seedExpired(t)
		before, err := os.ReadFile(path)
		Expect(err).ToNot(HaveOccurred())

		refresher := &fakeRefresher{err: errors.New("503 Service Unavailable")}
		inner := &countingTokenSource{token: freshToken("interactive")}

		_, err = NewRefreshingCachingTokenSource(path, refresher, inner).Token()

		Expect(err).To(MatchError(ContainSubstring("503")))
		Expect(inner.calls).To(Equal(0))

		after, err := os.ReadFile(path)
		Expect(err).ToNot(HaveOccurred())
		Expect(after).To(Equal(before))
	})

	// A cache written before refresh tokens existed has to keep working.
	t.Run("Logs in again when the cached token predates refresh tokens", func(t *testing.T) {
		RegisterTestingT(t)

		path := filepath.Join(t.TempDir(), "token.json")
		writeCache(t, path, `{"access_token":"legacy","expiry":"2000-01-01T00:00:00Z","expires_in":86400}`)

		refresher := &fakeRefresher{}
		inner := &countingTokenSource{token: freshToken("interactive")}

		token, err := NewRefreshingCachingTokenSource(path, refresher, inner).Token()

		Expect(err).ToNot(HaveOccurred())
		Expect(token.AccessToken).To(Equal("interactive"))
		Expect(inner.calls).To(Equal(1))
		Expect(refresher.seen).To(BeEmpty())
	})

	t.Run("Refreshes once under concurrent callers", func(t *testing.T) {
		RegisterTestingT(t)

		path := seedExpired(t)
		refresher := &fakeRefresher{token: &oauth2.Token{
			AccessToken:  "new-at",
			RefreshToken: "new-rt",
			Expiry:       time.Now().Add(time.Hour),
		}}
		src := NewRefreshingCachingTokenSource(path, refresher, &countingTokenSource{})

		var wg sync.WaitGroup
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _ = src.Token()
			}()
		}
		wg.Wait()

		Expect(refresher.seen).To(HaveLen(1))
	})

	// The regression test for the layering: oauth2.ReuseTokenSource
	// memoizes in memory, so a refresher above it would never reach disk.
	t.Run("Reaches the disk through oauth2.ReuseTokenSource", func(t *testing.T) {
		RegisterTestingT(t)

		path := seedExpired(t)
		refresher := &fakeRefresher{token: &oauth2.Token{
			AccessToken:  "new-at",
			RefreshToken: "new-rt",
			Expiry:       time.Now().Add(time.Hour),
		}}
		src := oauth2.ReuseTokenSource(nil,
			NewRefreshingCachingTokenSource(path, refresher, &countingTokenSource{}))

		_, err := src.Token()
		Expect(err).ToNot(HaveOccurred())

		stored, err := loadTokenFromDisk(path)
		Expect(err).ToNot(HaveOccurred())
		Expect(stored.AccessToken).To(Equal("new-at"))
		Expect(stored.RefreshToken).To(Equal("new-rt"))
	})
}

func TestSaveTokenToDisk(t *testing.T) {
	// The file holds a bearer credential; the default 0644 handed it to
	// every account on the machine.
	t.Run("Writes the token readable by its owner only", func(t *testing.T) {
		RegisterTestingT(t)

		path := filepath.Join(t.TempDir(), "token.json")
		Expect(saveTokenToDisk(path, freshToken("secret"))).To(Succeed())

		info, err := os.Stat(path)
		Expect(err).ToNot(HaveOccurred())
		Expect(info.Mode().Perm()).To(Equal(os.FileMode(tokenFileMode)))
	})

	t.Run("Tightens the permissions of a cache written by an older version", func(t *testing.T) {
		RegisterTestingT(t)

		path := filepath.Join(t.TempDir(), "token.json")
		writeCache(t, path, `{"access_token":"old"}`)

		Expect(saveTokenToDisk(path, freshToken("new"))).To(Succeed())

		info, err := os.Stat(path)
		Expect(err).ToNot(HaveOccurred())
		Expect(info.Mode().Perm()).To(Equal(os.FileMode(tokenFileMode)))
	})

	t.Run("Creates the cache directory", func(t *testing.T) {
		RegisterTestingT(t)

		path := filepath.Join(t.TempDir(), "nested", "token.json")
		Expect(saveTokenToDisk(path, freshToken("secret"))).To(Succeed())
		Expect(path).To(BeAnExistingFile())
	})

	t.Run("Leaves no temporary files behind", func(t *testing.T) {
		RegisterTestingT(t)

		dir := t.TempDir()
		Expect(saveTokenToDisk(filepath.Join(dir, "token.json"), freshToken("secret"))).To(Succeed())

		entries, err := os.ReadDir(dir)
		Expect(err).ToNot(HaveOccurred())
		Expect(entries).To(HaveLen(1))
		Expect(entries[0].Name()).To(Equal("token.json"))
	})

	t.Run("Round-trips a token", func(t *testing.T) {
		RegisterTestingT(t)

		path := filepath.Join(t.TempDir(), "token.json")
		want := freshToken("secret")
		want.RefreshToken = "refresh"
		Expect(saveTokenToDisk(path, want)).To(Succeed())

		got, err := loadTokenFromDisk(path)
		Expect(err).ToNot(HaveOccurred())
		Expect(got.AccessToken).To(Equal(want.AccessToken))
		Expect(got.RefreshToken).To(Equal(want.RefreshToken))
		Expect(got.TokenType).To(Equal(want.TokenType))
		Expect(got.Expiry).To(BeTemporally("~", want.Expiry, time.Second))
	})
}

func TestLoadTokenFromDisk(t *testing.T) {
	t.Run("Reports a missing cache as fs.ErrNotExist", func(t *testing.T) {
		RegisterTestingT(t)

		_, err := loadTokenFromDisk(filepath.Join(t.TempDir(), "absent.json"))
		Expect(err).To(MatchError(os.ErrNotExist))
	})

	t.Run("Names the file it could not parse", func(t *testing.T) {
		RegisterTestingT(t)

		path := filepath.Join(t.TempDir(), "token.json")
		writeCache(t, path, "{")

		_, err := loadTokenFromDisk(path)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(path))
		Expect(strings.Contains(err.Error(), "not exist")).To(BeFalse())
	})
}
