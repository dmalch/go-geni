package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"time"

	"github.com/skratchdot/open-golang/open"
)

const (
	// defaultCallbackPort is the port the OAuth callback listener binds
	// when the caller does not pick one. It has to match the Callback URL
	// registered with the Geni application, which is the only thing that
	// decides where Geni redirects — see authCodeURL for why the request
	// cannot carry a redirect_uri of its own.
	defaultCallbackPort = 8080

	// callbackPath is the path the callback listener serves.
	callbackPath = "/callback"

	// displayWeb asks Geni for the full-size, desktop-sized authorization
	// screen. Geni also accepts "mobile" (phone-sized), "desktop" (which
	// ignores redirect_uri and lands the token on an internal Geni page,
	// unreachable from a loopback listener) and "iframe" (canvas apps).
	displayWeb = "web"

	// defaultLoginTimeout bounds how long the flow waits for the user to
	// finish authorizing in the browser.
	defaultLoginTimeout = 5 * time.Minute

	// defaultTokenLifetime stands in when Geni's callback carries no
	// usable expires_in. It matches the 86400 seconds Geni documents.
	defaultTokenLifetime = 24 * time.Hour

	// callbackShutdownTimeout bounds the graceful shutdown of the
	// callback server before it is closed outright.
	callbackShutdownTimeout = 2 * time.Second
)

// errLoginTimedOut is the cause attached to the login context when the
// user does not finish authorizing in time.
var errLoginTimedOut = errors.New("timed out while waiting for a response")

// openBrowser is a seam so the login flow can be exercised without launching a
// real browser. Production code always uses open.Start.
var openBrowser = open.Start

// options are the settings shared by every browser-based flow.
type options struct {
	ctx      context.Context
	port     int
	timeout  time.Duration
	lifetime time.Duration
}

// Option configures a browser-based token source. The zero set of
// options reproduces the historical behavior: the callback listener binds
// port 8080 and Geni redirects to the callback URL registered with the
// application.
type Option func(*options)

// WithPort pins the port the OAuth callback listener binds; 0 asks the
// operating system for a free one.
//
// It must match the Callback URL registered with the Geni application:
// that single URL is the only thing deciding where Geni redirects, an
// application may register exactly one, and the request cannot carry a
// redirect_uri of its own (see authCodeURL). A port of 0 is therefore
// only useful in tests.
func WithPort(port int) Option {
	return func(o *options) { o.port = port }
}

// WithContext makes the login flow abort when ctx is canceled.
//
// The context lives on the token source rather than on Token() because
// oauth2.TokenSource fixes that signature, and this source is always
// reached through a wrapper (oauth2.ReuseTokenSource, the caching source)
// that types it as the interface — so a TokenContext method would never
// be called.
func WithContext(ctx context.Context) Option {
	return func(o *options) { o.ctx = ctx }
}

// WithTimeout bounds how long the flow waits for the browser callback.
// The default is five minutes.
func WithTimeout(d time.Duration) Option {
	return func(o *options) { o.timeout = d }
}

// WithTokenLifetime sets the lifetime assumed when Geni's callback
// carries no usable expires_in. The default is 24 hours.
func WithTokenLifetime(d time.Duration) Option {
	return func(o *options) { o.lifetime = d }
}

// newOptions applies opts on top of the defaults.
func newOptions(opts ...Option) options {
	o := options{
		ctx:      context.Background(),
		port:     defaultCallbackPort,
		timeout:  defaultLoginTimeout,
		lifetime: defaultTokenLifetime,
	}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// loopbackFlow runs the browser half of an OAuth flow: it serves a
// callback on the loopback interface, hands the authorization URL to a
// browser, and returns the query parameters Geni redirected with.
type loopbackFlow struct {
	options

	// boundPort is the port the callback listener actually bound, which
	// differs from port only when port is 0.
	boundPort int
}

// authorize runs the flow. authURL is called with the random state the
// callback is expected to echo back.
func (f *loopbackFlow) authorize(authURL func(state string) string) (url.Values, error) {
	state, err := newState()
	if err != nil {
		return nil, err
	}

	// Bind before anything is shown to the user: ListenAndServe would
	// report "address already in use" asynchronously, which used to open a
	// browser tab first and only then fail.
	listener, err := listenLoopback(f.ctx, f.port)
	if err != nil {
		return nil, fmt.Errorf("failed to start the OAuth callback listener on port %d: %w", f.port, err)
	}
	f.boundPort = listenerPort(listener)

	handler := &callback{
		expectedState: state,
		resultCh:      make(chan callbackResult, 1),
	}

	mux := http.NewServeMux()
	mux.HandleFunc(callbackPath, handler.handle)
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	// Every return path below runs this, so an interrupted or timed-out
	// login releases the port instead of holding it for the life of the
	// process.
	defer shutdownCallbackServer(server, listener)

	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			handler.deliver(callbackResult{err: fmt.Errorf("failed to start callback server: %w", err)})
		}
	}()

	ctx, cancel := context.WithTimeoutCause(f.ctx, f.timeout, errLoginTimedOut)
	defer cancel()
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	// Print the URL before handing it to the browser. `open.Start` gives the
	// caller no way to recover it, and the URL cannot be rebuilt afterwards:
	// `state` is random, single-use and lives only in this process. Without
	// this line there is nothing to fall back on when the default browser is
	// not the one holding the Geni session — a remote shell, a second Chrome
	// profile, or an agent driving a browser it does not own.
	url := authURL(state)
	_, _ = fmt.Fprintf(os.Stderr, "Open this URL to authorize:\n%s\n", url)
	_, _ = fmt.Fprintf(os.Stderr, "Waiting for the OAuth callback on http://localhost:%d%s\n", f.boundPort, callbackPath)

	// Opening a browser is a convenience, not a precondition: the callback
	// server is already listening and the URL is on screen, so a failure here
	// leaves the flow perfectly usable by hand.
	if err := openBrowser(url); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "could not open a browser automatically (%v) — open the URL above\n", err)
	}

	select {
	case result := <-handler.resultCh:
		return result.query, result.err
	case <-ctx.Done():
		return nil, loginContextError(ctx, f.ctx)
	}
}

// newState returns a cryptographically random state to protect against
// CSRF.
func newState() (string, error) {
	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return "", fmt.Errorf("failed to generate OAuth2 state: %w", err)
	}
	return hex.EncodeToString(stateBytes), nil
}

// listenLoopback binds the OAuth callback listener. It binds the loopback
// interface only: the callback carries an access token in its query
// string and has no business being reachable from the network. A port of
// 0 asks the operating system for a free one.
func listenLoopback(ctx context.Context, port int) (net.Listener, error) {
	var lc net.ListenConfig
	return lc.Listen(ctx, "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
}

// listenerPort returns the TCP port a listener bound, or 0 when the
// address is not a TCP one.
func listenerPort(listener net.Listener) int {
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0
	}
	return addr.Port
}

// shutdownCallbackServer stops the callback server and releases the port.
// Closing the listener as well covers the case where Serve never ran.
func shutdownCallbackServer(server *http.Server, listener net.Listener) {
	ctx, cancel := context.WithTimeout(context.Background(), callbackShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		_ = server.Close()
	}
	_ = listener.Close()
}

// loginContextError explains why the login context ended: the caller
// canceled it, the user pressed Ctrl-C, or the flow timed out.
func loginContextError(ctx, parent context.Context) error {
	switch {
	case errors.Is(context.Cause(ctx), errLoginTimedOut):
		return errLoginTimedOut
	case parent.Err() != nil:
		return fmt.Errorf("login canceled: %w", context.Cause(parent))
	default:
		return errors.New("interrupted")
	}
}

// callbackResult is what the browser callback produced: either the query
// parameters Geni redirected with, or the error that ended the flow.
type callbackResult struct {
	query url.Values
	err   error
}

type callback struct {
	expectedState string
	resultCh      chan callbackResult
	once          sync.Once
}

// deliver publishes the first result and ignores any later one. The
// channel is buffered, so the handler never blocks and a reloaded or
// prefetched callback cannot deadlock a goroutine.
func (handler *callback) deliver(result callbackResult) {
	handler.once.Do(func() { handler.resultCh <- result })
}

func (handler *callback) handle(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	if state := query.Get("state"); state != handler.expectedState {
		_, _ = fmt.Fprintln(w, "OAuth2 state mismatch. Possible CSRF attack. Please try again.")
		handler.deliver(callbackResult{err: errors.New("OAuth2 state parameter mismatch")})
		return
	}

	if code := query.Get("error"); code != "" {
		_, _ = fmt.Fprintln(w, "Login was not successful. You can close the browser and try again.")
		handler.deliver(callbackResult{err: authorizationError(code, query.Get("error_description"))})
		return
	}

	if query.Get("access_token") != "" || query.Get("code") != "" {
		_, _ = fmt.Fprintln(w, "Login was successful. You can close the browser and return to the command line.")
	} else {
		_, _ = fmt.Fprintln(w, "Login was not successful. You can close the browser and try again.")
	}
	handler.deliver(callbackResult{query: query})
}

// authorizationError turns Geni's error/error_description pair into an
// error. Geni sends status=unauthorized&message=user+canceled when the
// user declines, so both spellings are worth surfacing.
func authorizationError(code, description string) error {
	if description == "" {
		return fmt.Errorf("authorization failed: %s", code)
	}
	return fmt.Errorf("authorization failed: %s (%s)", code, description)
}
