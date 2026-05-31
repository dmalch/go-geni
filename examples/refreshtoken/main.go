// Command refreshtoken runs the same OAuth implicit-flow handshake as the
// terraform-provider-genealogy uses, then writes the resulting token to
// ~/.genealogy/geni_token.json (prod) or geni_sandbox_token.json (sandbox).
//
// Unlike auth.NewAuthTokenSource (which opens the URL via the OS default
// browser), this command also prints the URL to stderr so a separate
// automation tool (e.g. playwright-cli attached to the user's logged-in
// Chrome) can drive the navigation.
//
// Run:
//
//	go run ./examples/refreshtoken            # production
//	go run ./examples/refreshtoken -sandbox   # sandbox
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"time"

	geni "github.com/dmalch/go-geni"
	"golang.org/x/oauth2"
)

func main() {
	sandbox := flag.Bool("sandbox", false, "use sandbox.geni.com instead of production")
	noOpen := flag.Bool("no-open", true, "don't open browser automatically (always print URL)")
	flag.Parse()
	_ = noOpen // currently always behaves as no-open=true

	clientID := "1855"
	if *sandbox {
		clientID = "8"
	}

	cfg := &oauth2.Config{
		ClientID: clientID,
		Endpoint: oauth2.Endpoint{
			AuthURL: geni.BaseURL(*sandbox) + "platform/oauth/authorize",
		},
	}

	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		log.Fatalf("rand: %v", err)
	}
	state := hex.EncodeToString(stateBytes)

	authURL := cfg.AuthCodeURL(state, oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("response_type", "token"),
		oauth2.SetAuthURLParam("display", "mobile"),
	)

	cachePath, err := tokenCachePath(*sandbox)
	if err != nil {
		log.Fatalf("cache path: %v", err)
	}
	fmt.Fprintf(os.Stderr, "auth URL:\n  %s\n", authURL)
	fmt.Fprintf(os.Stderr, "expected state: %s\n", state)
	fmt.Fprintf(os.Stderr, "will save to:   %s\n", cachePath)
	fmt.Fprintf(os.Stderr, "callback server starting on :8080 ...\n")

	tokenCh := make(chan *oauth2.Token, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()

	// The OAuth implicit response_type=token returns the token in the URL
	// fragment, which the server cannot see directly. The provider's auth
	// package serves a small HTML page that JS-redirects to /callback?<fragment as query>.
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, `<html><body><script>
var h = window.location.hash;
if (h && h[0]=='#') { window.location.replace('/callback?' + h.substring(1)); }
</script>OK</body></html>`)
	})

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		got := query.Get("state")
		if got != state {
			errCh <- fmt.Errorf("state mismatch: got %q want %q", got, state)
			http.Error(w, "state mismatch", http.StatusBadRequest)
			return
		}
		access := query.Get("access_token")
		if access == "" {
			errCh <- errors.New("no access_token in callback")
			http.Error(w, "missing token", http.StatusBadRequest)
			return
		}
		expiresIn := query.Get("expires_in")
		secs, _ := strconv.ParseInt(expiresIn, 10, 64)
		expiry := time.Now().Add(time.Duration(secs) * time.Second)
		t := &oauth2.Token{
			AccessToken: access,
			TokenType:   "Bearer",
			Expiry:      expiry,
		}
		tokenCh <- t
		_, _ = fmt.Fprint(w, "token captured; you can close this tab")
	})

	server := &http.Server{Addr: ":8080", Handler: mux}
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	fmt.Fprintf(os.Stderr, "\nNavigate the authenticated browser to the URL above.\n")
	fmt.Fprintf(os.Stderr, "Geni will redirect to http://localhost:8080/ with the access_token in the fragment;\n")
	fmt.Fprintf(os.Stderr, "this tool serves a tiny page that forwards it to /callback.\n\n")

	select {
	case t := <-tokenCh:
		_ = server.Shutdown(context.Background())
		if err := saveToken(cachePath, t); err != nil {
			log.Fatalf("save token: %v", err)
		}
		fmt.Fprintf(os.Stderr, "✓ token saved (expires %s)\n", t.Expiry.Format(time.RFC3339))
	case err := <-errCh:
		_ = server.Shutdown(context.Background())
		log.Fatalf("auth error: %v", err)
	case <-time.After(5 * time.Minute):
		_ = server.Shutdown(context.Background())
		log.Fatalf("timeout waiting for browser callback")
	}
}

func tokenCachePath(sandbox bool) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	name := "geni_token.json"
	if sandbox {
		name = "geni_sandbox_token.json"
	}
	return path.Join(home, ".genealogy", name), nil
}

func saveToken(p string, t *oauth2.Token) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(t)
}
