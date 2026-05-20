package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/skratchdot/open-golang/open"
	"golang.org/x/oauth2"
)

// authTokenSource implements oauth2.TokenSource.
type authTokenSource struct {
	config *oauth2.Config
}

func NewAuthTokenSource(config *oauth2.Config) *authTokenSource {
	return &authTokenSource{
		config: config,
	}
}

// Token retrieves a new token, performing the OAuth flow if necessary.
func (a *authTokenSource) Token() (*oauth2.Token, error) {
	sigIntCh := make(chan os.Signal, 1)
	signal.Notify(sigIntCh, os.Interrupt)
	defer signal.Stop(sigIntCh)

	// Generate a cryptographically random state to protect against CSRF.
	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return nil, fmt.Errorf("failed to generate OAuth2 state: %w", err)
	}
	state := hex.EncodeToString(stateBytes)

	// Start a local server to handle the callback
	callbackHandler := &callback{
		expectedState: state,
		shutdownCh:    make(chan error),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", callbackHandler.handle)
	server := &http.Server{Addr: ":8080", Handler: mux}
	go func() {
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			callbackHandler.shutdownCh <- fmt.Errorf("failed to start callback server: %w", err)
		}
	}()

	authURL := a.config.AuthCodeURL(state, oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("response_type", "token"),
		oauth2.SetAuthURLParam("display", "mobile"),
	)

	// Open the URL in the default browser
	err := open.Start(authURL)
	if err != nil {
		return nil, err
	}

	// Wait for the token to be set by the callback handler, for SIGINT to be
	// received, or for up to 5 minutes.
	select {
	case err := <-callbackHandler.shutdownCh:
		if err != nil {
			return nil, err
		}

		err = server.Shutdown(context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to shutdown the server: %w", err)
		}

		if callbackHandler.accessToken == "" {
			return nil, errors.New("no authentication access token was received")
		}

		expiresIn, err := strconv.ParseInt(callbackHandler.expiresIn, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse expires_in: %w", err)
		}

		return &oauth2.Token{
			AccessToken: callbackHandler.accessToken,
			ExpiresIn:   expiresIn,
			Expiry:      time.Now().Add(time.Duration(expiresIn) * time.Second),
		}, nil
	case <-sigIntCh:
		return nil, errors.New("interrupted")
	case <-time.After(5 * time.Minute):
		return nil, errors.New("timed out while waiting for a response")
	}
}

type callback struct {
	expectedState string
	accessToken   string
	expiresIn     string
	shutdownCh    chan error
}

func (handler *callback) handle(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	if state := query.Get("state"); state != handler.expectedState {
		_, _ = fmt.Fprintln(w, "OAuth2 state mismatch. Possible CSRF attack. Please try again.")
		handler.shutdownCh <- errors.New("OAuth2 state parameter mismatch")
		return
	}

	accessToken := query.Get("access_token")
	if accessToken != "" {
		handler.accessToken = accessToken
		handler.expiresIn = query.Get("expires_in")
		_, _ = fmt.Fprintln(w, "Login was successful. You can close the browser and return to the command line.")
	} else {
		_, _ = fmt.Fprintln(w, "Login was not successful. You can close the browser and try again.")
	}
	handler.shutdownCh <- nil
}
