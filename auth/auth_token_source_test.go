package auth

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	. "github.com/onsi/gomega"
	"golang.org/x/oauth2"
)

func TestCallbackHandle(t *testing.T) {
	t.Run("Accepts token when state matches", func(t *testing.T) {
		RegisterTestingT(t)

		handler := &callback{
			expectedState: "valid-state",
			shutdownCh:    make(chan error, 1),
		}

		q := make(url.Values)
		q.Set("state", "valid-state")
		q.Set("access_token", "my-token")
		q.Set("expires_in", "3600")
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/callback?"+q.Encode(), nil)
		rec := httptest.NewRecorder()

		handler.handle(rec, req)

		shutdownErr := <-handler.shutdownCh
		Expect(shutdownErr).ToNot(HaveOccurred())
		Expect(handler.accessToken).To(Equal("my-token"))
		Expect(handler.expiresIn).To(Equal("3600"))
		Expect(rec.Body.String()).To(ContainSubstring("Login was successful"))
	})

	t.Run("Rejects callback when state does not match", func(t *testing.T) {
		RegisterTestingT(t)

		handler := &callback{
			expectedState: "valid-state",
			shutdownCh:    make(chan error, 1),
		}

		q := make(url.Values)
		q.Set("state", "wrong-state")
		q.Set("access_token", "my-token")
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/callback?"+q.Encode(), nil)
		rec := httptest.NewRecorder()

		handler.handle(rec, req)

		shutdownErr := <-handler.shutdownCh
		Expect(shutdownErr).To(HaveOccurred())
		Expect(shutdownErr.Error()).To(ContainSubstring("state parameter mismatch"))
		Expect(handler.accessToken).To(BeEmpty())
		Expect(rec.Body.String()).To(ContainSubstring("CSRF"))
	})

	t.Run("Rejects callback when state is missing", func(t *testing.T) {
		RegisterTestingT(t)

		handler := &callback{
			expectedState: "valid-state",
			shutdownCh:    make(chan error, 1),
		}

		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/callback?access_token=my-token", nil)
		rec := httptest.NewRecorder()

		handler.handle(rec, req)

		shutdownErr := <-handler.shutdownCh
		Expect(shutdownErr).To(HaveOccurred())
		Expect(handler.accessToken).To(BeEmpty())
	})

	t.Run("Reports unsuccessful login when token is empty but state matches", func(t *testing.T) {
		RegisterTestingT(t)

		handler := &callback{
			expectedState: "valid-state",
			shutdownCh:    make(chan error, 1),
		}

		q := make(url.Values)
		q.Set("state", "valid-state")
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/callback?"+q.Encode(), nil)
		rec := httptest.NewRecorder()

		handler.handle(rec, req)

		shutdownErr := <-handler.shutdownCh
		Expect(shutdownErr).ToNot(HaveOccurred())
		Expect(handler.accessToken).To(BeEmpty())
		Expect(rec.Body.String()).To(ContainSubstring("Login was not successful"))
	})
}

func TestNewAuthTokenSource(t *testing.T) {
	t.Run("Creates token source with config", func(t *testing.T) {
		RegisterTestingT(t)

		src := NewAuthTokenSource(nil)
		Expect(src).ToNot(BeNil())
		Expect(src.config).To(BeNil())
	})
}

// captureStderr swaps os.Stderr for a pipe, runs fn, and returns what was
// written. The pipe is drained on a goroutine so a chatty fn cannot deadlock on
// a full pipe buffer.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	orig := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = orig }()

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()
	_ = w.Close()
	return <-done
}

func TestTokenPrintsAuthURL(t *testing.T) {
	// `open.Start` hands the URL to the default browser and gives the caller no
	// way to recover it, and it cannot be rebuilt afterwards because `state` is
	// random and lives only in this process. Printing it is the only fallback
	// when the default browser is not the one holding the Geni session.
	t.Run("Prints the authorization URL to stderr", func(t *testing.T) {
		RegisterTestingT(t)

		src := NewAuthTokenSource(&oauth2.Config{
			ClientID: "1855",
			Endpoint: oauth2.Endpoint{AuthURL: "https://www.geni.com/platform/oauth/authorize"},
		})

		// Stand in for the browser: complete the flow by calling the local
		// callback with the state carried in the URL we were handed.
		// The callback must be fired from a goroutine: `shutdownCh` is
		// unbuffered, so the handler blocks until Token() reaches its select —
		// which cannot happen while openBrowser is still on the stack.
		var opened string
		restore := stubOpenBrowser(func(authURL string) error {
			opened = authURL
			u, err := url.Parse(authURL)
			if err != nil {
				return err
			}
			go func() {
				q := make(url.Values)
				q.Set("state", u.Query().Get("state"))
				q.Set("access_token", "my-token")
				q.Set("expires_in", "3600")
				resp, hErr := http.Get("http://localhost:8080/callback?" + q.Encode()) //nolint:noctx // test helper
				if hErr == nil {
					_ = resp.Body.Close()
				}
			}()
			return nil
		})
		defer restore()

		var token *oauth2.Token
		var tokenErr error
		stderr := captureStderr(t, func() { token, tokenErr = src.Token() })

		Expect(tokenErr).ToNot(HaveOccurred())
		Expect(token.AccessToken).To(Equal("my-token"))
		Expect(stderr).To(ContainSubstring(opened))
		Expect(stderr).To(ContainSubstring("state="))
	})

	t.Run("Keeps waiting when no browser can be opened", func(t *testing.T) {
		RegisterTestingT(t)

		src := NewAuthTokenSource(&oauth2.Config{
			ClientID: "1855",
			Endpoint: oauth2.Endpoint{AuthURL: "https://www.geni.com/platform/oauth/authorize"},
		})

		// A browser that refuses to launch must not abort the flow: the URL is
		// already on screen and the callback server is already listening, so
		// the user can finish by hand.
		restore := stubOpenBrowser(func(authURL string) error {
			u, err := url.Parse(authURL)
			if err != nil {
				return err
			}
			go func() {
				q := make(url.Values)
				q.Set("state", u.Query().Get("state"))
				q.Set("access_token", "hand-opened")
				q.Set("expires_in", "3600")
				resp, hErr := http.Get("http://localhost:8080/callback?" + q.Encode()) //nolint:noctx // test helper
				if hErr == nil {
					_ = resp.Body.Close()
				}
			}()
			return errors.New("no browser available")
		})
		defer restore()

		var token *oauth2.Token
		var tokenErr error
		stderr := captureStderr(t, func() { token, tokenErr = src.Token() })

		Expect(tokenErr).ToNot(HaveOccurred())
		Expect(token.AccessToken).To(Equal("hand-opened"))
		Expect(stderr).To(ContainSubstring("could not open a browser automatically"))
	})
}

// stubOpenBrowser replaces the browser seam and returns a restore func.
func stubOpenBrowser(fn func(string) error) func() {
	orig := openBrowser
	openBrowser = fn
	return func() { openBrowser = orig }
}
