package auth

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"golang.org/x/oauth2"
)

// expectedState is the state the callback handler under test is primed
// with.
const expectedState = "valid-state"

func newCallback() *callback {
	return &callback{
		expectedState: expectedState,
		resultCh:      make(chan callbackResult, 1),
	}
}

func TestCallbackHandle(t *testing.T) {
	t.Run("Accepts token when state matches", func(t *testing.T) {
		RegisterTestingT(t)

		handler := newCallback()

		q := make(url.Values)
		q.Set("state", "valid-state")
		q.Set("access_token", "my-token")
		q.Set("expires_in", "3600")
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/callback?"+q.Encode(), nil)
		rec := httptest.NewRecorder()

		handler.handle(rec, req)

		result := <-handler.resultCh
		Expect(result.err).ToNot(HaveOccurred())
		Expect(result.query.Get("access_token")).To(Equal("my-token"))
		Expect(result.query.Get("expires_in")).To(Equal("3600"))
		Expect(rec.Body.String()).To(ContainSubstring("Login was successful"))
	})

	t.Run("Rejects callback when state does not match", func(t *testing.T) {
		RegisterTestingT(t)

		handler := newCallback()

		q := make(url.Values)
		q.Set("state", "wrong-state")
		q.Set("access_token", "my-token")
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/callback?"+q.Encode(), nil)
		rec := httptest.NewRecorder()

		handler.handle(rec, req)

		result := <-handler.resultCh
		Expect(result.err).To(HaveOccurred())
		Expect(result.err.Error()).To(ContainSubstring("state parameter mismatch"))
		Expect(result.query).To(BeNil())
		Expect(rec.Body.String()).To(ContainSubstring("CSRF"))
	})

	t.Run("Rejects callback when state is missing", func(t *testing.T) {
		RegisterTestingT(t)

		handler := newCallback()

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/callback?access_token=my-token", nil)
		rec := httptest.NewRecorder()

		handler.handle(rec, req)

		result := <-handler.resultCh
		Expect(result.err).To(HaveOccurred())
		Expect(result.query).To(BeNil())
	})

	t.Run("Reports unsuccessful login when token is empty but state matches", func(t *testing.T) {
		RegisterTestingT(t)

		handler := newCallback()

		q := make(url.Values)
		q.Set("state", "valid-state")
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/callback?"+q.Encode(), nil)
		rec := httptest.NewRecorder()

		handler.handle(rec, req)

		result := <-handler.resultCh
		Expect(result.err).ToNot(HaveOccurred())
		Expect(result.query.Get("access_token")).To(BeEmpty())
		Expect(rec.Body.String()).To(ContainSubstring("Login was not successful"))
	})

	// Geni redirects with error/error_description when the user declines
	// on the consent screen. Reporting that beats waiting out the timeout.
	t.Run("Surfaces an authorization error from the consent screen", func(t *testing.T) {
		RegisterTestingT(t)

		handler := newCallback()

		q := make(url.Values)
		q.Set("state", "valid-state")
		q.Set("error", "unauthorized")
		q.Set("error_description", "user canceled")
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/callback?"+q.Encode(), nil)
		rec := httptest.NewRecorder()

		handler.handle(rec, req)

		result := <-handler.resultCh
		Expect(result.err).To(HaveOccurred())
		Expect(result.err.Error()).To(ContainSubstring("unauthorized"))
		Expect(result.err.Error()).To(ContainSubstring("user canceled"))
	})

	// A reloaded or prefetched callback must not block a goroutine on a
	// channel nobody is reading any more.
	t.Run("Delivers only the first callback", func(t *testing.T) {
		RegisterTestingT(t)

		handler := newCallback()

		for _, token := range []string{"first", "second"} {
			q := make(url.Values)
			q.Set("state", "valid-state")
			q.Set("access_token", token)
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/callback?"+q.Encode(), nil)
			handler.handle(httptest.NewRecorder(), req)
		}

		result := <-handler.resultCh
		Expect(result.query.Get("access_token")).To(Equal("first"))
		Expect(handler.resultCh).To(BeEmpty())
	})
}

func TestNewAuthTokenSource(t *testing.T) {
	t.Run("Creates token source with config", func(t *testing.T) {
		RegisterTestingT(t)

		src := NewAuthTokenSource(nil)
		Expect(src).ToNot(BeNil())
		Expect(src.config).To(BeNil())
	})

	t.Run("Defaults to the registered callback port", func(t *testing.T) {
		RegisterTestingT(t)

		src := NewAuthTokenSource(nil)
		Expect(src.port).To(Equal(defaultCallbackPort))
		Expect(src.timeout).To(Equal(defaultLoginTimeout))
		Expect(src.lifetime).To(Equal(defaultTokenLifetime))
	})

	t.Run("Applies options", func(t *testing.T) {
		RegisterTestingT(t)

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		src := NewAuthTokenSource(nil,
			WithPort(0),
			WithContext(ctx),
			WithTimeout(time.Second),
			WithTokenLifetime(time.Hour),
		)
		Expect(src.port).To(Equal(0))
		Expect(src.ctx).To(Equal(ctx))
		Expect(src.timeout).To(Equal(time.Second))
		Expect(src.lifetime).To(Equal(time.Hour))
	})
}

func TestAuthCodeURL(t *testing.T) {
	newSource := func(opts ...Option) *authTokenSource {
		return NewAuthTokenSource(&oauth2.Config{
			ClientID: "1855",
			Endpoint: oauth2.Endpoint{AuthURL: "https://www.geni.com/platform/oauth/authorize"},
		}, opts...)
	}

	t.Run("Carries the parameters Geni's client-side flow needs", func(t *testing.T) {
		RegisterTestingT(t)

		q := queryOf(t, newSource().authCodeURL("some-state"))
		Expect(q.Get("display")).To(Equal("mobile"))
		Expect(q.Get("response_type")).To(Equal("token"))
		Expect(q.Get("client_id")).To(Equal("1855"))
		Expect(q.Get("state")).To(Equal("some-state"))
	})

	// Geni answers an authorization request that carries an explicit
	// redirect_uri with 403, even when the value is exactly the one
	// registered with the application. The redirect target is always the
	// registered Callback URL.
	t.Run("Never sends a redirect_uri", func(t *testing.T) {
		RegisterTestingT(t)

		Expect(queryOf(t, newSource().authCodeURL("some-state")).Has("redirect_uri")).To(BeFalse())
		Expect(queryOf(t, newSource(WithPort(9999)).authCodeURL("some-state")).Has("redirect_uri")).To(BeFalse())
	})
}

func TestListenLoopback(t *testing.T) {
	// The callback carries an access token in its query string, so the
	// listener has no business being reachable from the network.
	t.Run("Binds the loopback interface on a free port", func(t *testing.T) {
		RegisterTestingT(t)

		listener, err := listenLoopback(t.Context(), 0)
		Expect(err).ToNot(HaveOccurred())
		defer func() { _ = listener.Close() }()

		addr, ok := listener.Addr().(*net.TCPAddr)
		Expect(ok).To(BeTrue())
		Expect(addr.IP.IsLoopback()).To(BeTrue())
		Expect(addr.Port).ToNot(Equal(0))
	})

	t.Run("Reports a port that is already taken", func(t *testing.T) {
		RegisterTestingT(t)

		taken, err := listenLoopback(t.Context(), 0)
		Expect(err).ToNot(HaveOccurred())
		defer func() { _ = taken.Close() }()

		_, err = listenLoopback(t.Context(), listenerPort(taken))
		Expect(err).To(HaveOccurred())
	})
}

func TestToken(t *testing.T) {
	// `open.Start` hands the URL to the default browser and gives the caller no
	// way to recover it, and it cannot be rebuilt afterwards because `state` is
	// random and lives only in this process. Printing it is the only fallback
	// when the default browser is not the one holding the Geni session.
	t.Run("Prints the authorization URL to stderr", func(t *testing.T) {
		RegisterTestingT(t)

		src := ephemeralSource()
		var opened string
		restore := stubOpenBrowser(respondToCallback(&src.loopbackFlow, &opened, url.Values{
			"access_token": {"my-token"},
			"expires_in":   {"3600"},
		}))
		defer restore()

		var token *oauth2.Token
		var tokenErr error
		stderr := captureStderr(t, func() { token, tokenErr = src.Token() })

		Expect(tokenErr).ToNot(HaveOccurred())
		Expect(token.AccessToken).To(Equal("my-token"))
		Expect(token.TokenType).To(Equal("Bearer"))
		Expect(token.ExpiresIn).To(Equal(int64(3600)))
		Expect(stderr).To(ContainSubstring(opened))
		Expect(stderr).To(ContainSubstring("state="))
	})

	// The callback address is worth printing too: a login that hangs is
	// otherwise indistinguishable from one whose browser never arrived.
	t.Run("Prints the callback address it is waiting on", func(t *testing.T) {
		RegisterTestingT(t)

		src := ephemeralSource()
		var opened string
		restore := stubOpenBrowser(respondToCallback(&src.loopbackFlow, &opened, url.Values{
			"access_token": {"my-token"},
			"expires_in":   {"3600"},
		}))
		defer restore()

		var tokenErr error
		stderr := captureStderr(t, func() { _, tokenErr = src.Token() })

		Expect(tokenErr).ToNot(HaveOccurred())
		Expect(stderr).To(ContainSubstring(fmt.Sprintf("http://localhost:%d/callback", src.boundPort)))
	})

	t.Run("Keeps waiting when no browser can be opened", func(t *testing.T) {
		RegisterTestingT(t)

		// A browser that refuses to launch must not abort the flow: the URL is
		// already on screen and the callback server is already listening, so
		// the user can finish by hand.
		src := ephemeralSource()
		var opened string
		respond := respondToCallback(&src.loopbackFlow, &opened, url.Values{
			"access_token": {"hand-opened"},
			"expires_in":   {"3600"},
		})
		restore := stubOpenBrowser(func(authURL string) error {
			_ = respond(authURL)
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

	t.Run("Binds the port it was told to", func(t *testing.T) {
		RegisterTestingT(t)

		src := ephemeralSource()
		var opened string
		restore := stubOpenBrowser(respondToCallback(&src.loopbackFlow, &opened, url.Values{
			"access_token": {"my-token"},
			"expires_in":   {"3600"},
		}))
		defer restore()

		var tokenErr error
		captureStderr(t, func() { _, tokenErr = src.Token() })

		Expect(tokenErr).ToNot(HaveOccurred())
		Expect(src.boundPort).ToNot(Equal(0))
		Expect(src.boundPort).ToNot(Equal(defaultCallbackPort))
	})

	// A login that ends without a token must still release the port. A
	// successful re-bind is a direct observation of that; goroutine counts
	// are not.
	t.Run("Releases the port when it times out", func(t *testing.T) {
		RegisterTestingT(t)

		restore := stubOpenBrowser(func(string) error { return nil })
		defer restore()

		src := NewAuthTokenSource(geniConfig(), WithPort(0), WithTimeout(50*time.Millisecond))

		var tokenErr error
		captureStderr(t, func() { _, tokenErr = src.Token() })

		Expect(tokenErr).To(MatchError(errLoginTimedOut))
		expectPortFree(t, src.boundPort)
	})

	t.Run("Releases the port on a state mismatch", func(t *testing.T) {
		RegisterTestingT(t)

		src := ephemeralSource()
		var opened string
		restore := stubOpenBrowser(respondToCallbackWithState(&src.loopbackFlow, &opened, "wrong-state", url.Values{
			"access_token": {"my-token"},
		}))
		defer restore()

		var tokenErr error
		captureStderr(t, func() { _, tokenErr = src.Token() })

		Expect(tokenErr).To(MatchError(ContainSubstring("state parameter mismatch")))
		expectPortFree(t, src.boundPort)
	})

	t.Run("Stops when the caller cancels the context", func(t *testing.T) {
		RegisterTestingT(t)

		ctx, cancel := context.WithCancel(t.Context())
		restore := stubOpenBrowser(func(string) error {
			cancel()
			return nil
		})
		defer restore()

		src := NewAuthTokenSource(geniConfig(), WithPort(0), WithContext(ctx))

		var tokenErr error
		captureStderr(t, func() { _, tokenErr = src.Token() })

		Expect(tokenErr).To(MatchError(ContainSubstring("canceled")))
		expectPortFree(t, src.boundPort)
	})

	// Geni redirects with error/error_description when the user declines,
	// which beats waiting out the five-minute timeout.
	t.Run("Reports a declined authorization", func(t *testing.T) {
		RegisterTestingT(t)

		src := ephemeralSource()
		var opened string
		restore := stubOpenBrowser(respondToCallback(&src.loopbackFlow, &opened, url.Values{
			"error":             {"unauthorized"},
			"error_description": {"user canceled"},
		}))
		defer restore()

		var tokenErr error
		captureStderr(t, func() { _, tokenErr = src.Token() })

		Expect(tokenErr).To(MatchError(ContainSubstring("user canceled")))
	})

	t.Run("Fails when the callback carries no access token", func(t *testing.T) {
		RegisterTestingT(t)

		src := ephemeralSource()
		var opened string
		restore := stubOpenBrowser(respondToCallback(&src.loopbackFlow, &opened, url.Values{}))
		defer restore()

		var tokenErr error
		captureStderr(t, func() { _, tokenErr = src.Token() })

		Expect(tokenErr).To(MatchError(ContainSubstring("no authentication access token was received")))
	})

	// Losing a valid token over a missing lifetime would be a poor trade.
	// Leaving Expiry zero would be worse: oauth2 reads that as "never
	// expires" and the cache would serve a dead token forever.
	t.Run("Assumes a lifetime when expires_in is missing", func(t *testing.T) {
		RegisterTestingT(t)

		src := NewAuthTokenSource(geniConfig(), WithPort(0), WithTokenLifetime(time.Hour))
		var opened string
		restore := stubOpenBrowser(respondToCallback(&src.loopbackFlow, &opened, url.Values{
			"access_token": {"my-token"},
		}))
		defer restore()

		var token *oauth2.Token
		var tokenErr error
		logs := captureSlog(t, func() {
			captureStderr(t, func() { token, tokenErr = src.Token() })
		})

		Expect(tokenErr).ToNot(HaveOccurred())
		Expect(token.ExpiresIn).To(Equal(int64(3600)))
		Expect(token.Expiry).To(BeTemporally("~", time.Now().Add(time.Hour), time.Minute))
		Expect(logs).To(ContainSubstring("expires_in"))
	})

	t.Run("Fails before opening a browser when the port is taken", func(t *testing.T) {
		RegisterTestingT(t)

		taken, err := listenLoopback(t.Context(), 0)
		Expect(err).ToNot(HaveOccurred())
		defer func() { _ = taken.Close() }()

		opened := false
		restore := stubOpenBrowser(func(string) error {
			opened = true
			return nil
		})
		defer restore()

		src := NewAuthTokenSource(geniConfig(), WithPort(listenerPort(taken)))

		var tokenErr error
		captureStderr(t, func() { _, tokenErr = src.Token() })

		Expect(tokenErr).To(MatchError(ContainSubstring("callback listener")))
		Expect(opened).To(BeFalse())
	})
}

func geniConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID: "1855",
		Endpoint: oauth2.Endpoint{AuthURL: "https://www.geni.com/platform/oauth/authorize"},
	}
}

// ephemeralSource builds a token source that binds a free port, so tests
// never contend over the registered one.
func ephemeralSource() *authTokenSource {
	return NewAuthTokenSource(geniConfig(), WithPort(0))
}

// respondToCallback returns an openBrowser stub that stands in for the
// browser: it records the authorization URL, then calls the callback the
// source is listening on, echoing the state it was handed.
func respondToCallback(flow *loopbackFlow, opened *string, params url.Values) func(string) error {
	return respondToCallbackWithState(flow, opened, "", params)
}

// respondToCallbackWithState is respondToCallback with an explicit state,
// so a mismatch can be simulated. An empty state means "echo the one from
// the authorization URL".
func respondToCallbackWithState(flow *loopbackFlow, opened *string, state string, params url.Values) func(string) error {
	return func(authURL string) error {
		*opened = authURL

		u, err := url.Parse(authURL)
		if err != nil {
			return err
		}

		query := make(url.Values, len(params)+1)
		maps.Copy(query, params)
		if state == "" {
			state = u.Query().Get("state")
		}
		query.Set("state", state)

		// The result channel is buffered, so the handler never blocks and
		// this can run inline rather than from a goroutine.
		callbackURL := fmt.Sprintf("http://127.0.0.1:%d%s?%s", flow.boundPort, callbackPath, query.Encode())
		resp, err := http.Get(callbackURL) //nolint:noctx // test helper
		if err != nil {
			return err
		}
		return resp.Body.Close()
	}
}

// expectPortFree asserts that the callback listener released its port.
func expectPortFree(t *testing.T, port int) {
	t.Helper()

	listener, err := listenLoopback(t.Context(), port)
	if err != nil {
		t.Fatalf("port %d was not released: %v", port, err)
	}
	_ = listener.Close()
}

func queryOf(t *testing.T, rawURL string) url.Values {
	t.Helper()

	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("failed to parse %q: %v", rawURL, err)
	}
	return u.Query()
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

// captureSlog redirects the default logger for the duration of fn and
// returns what it logged. Swapping os.Stderr is not enough: the default
// handler captured the original file when it was built.
func captureSlog(t *testing.T, fn func()) string {
	t.Helper()

	var buf bytes.Buffer
	orig := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(orig) })

	fn()
	return buf.String()
}

// stubOpenBrowser replaces the browser seam and returns a restore func.
func stubOpenBrowser(fn func(string) error) func() {
	orig := openBrowser
	openBrowser = fn
	return func() { openBrowser = orig }
}
