package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/dmalch/go-geni/web/browsercookies"
)

// userConfig is the persisted CLI configuration stored at
// ~/.genealogy/config.json. Fields are optional; an empty value means
// "not set". Future settings can be added without changing the file
// format — unknown fields are tolerated by encoding/json.
type userConfig struct {
	Browser string `json:"browser,omitempty"`
	Version int    `json:"version,omitempty"`

	// Prod and Sandbox hold the OAuth application to authenticate as.
	// A client secret unlocks Geni's server-side flow, which returns a
	// refresh token and spares a browser round trip every day.
	Prod    oauthApp `json:"prod,omitzero"`
	Sandbox oauthApp `json:"sandbox,omitzero"`
}

// oauthApp is a registered Geni application. It stays comparable so the
// zero-config check in saveUserConfig keeps working.
type oauthApp struct {
	ClientID     string `json:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
}

// userConfigPath returns the path of the persisted CLI config file.
// Lives alongside the OAuth token cache.
func userConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home dir: %w", err)
	}
	return filepath.Join(home, ".genealogy", "config.json"), nil
}

// loadUserConfig reads ~/.genealogy/config.json. A missing file is
// not an error — it returns the zero-value config.
func loadUserConfig() (userConfig, error) {
	p, err := userConfigPath()
	if err != nil {
		return userConfig{}, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return userConfig{}, nil
		}
		return userConfig{}, fmt.Errorf("read %s: %w", p, err)
	}
	var c userConfig
	if err := json.Unmarshal(b, &c); err != nil {
		return userConfig{}, fmt.Errorf("parse %s: %w", p, err)
	}
	return c, nil
}

// saveUserConfig writes c to ~/.genealogy/config.json (mode 0600,
// creating the directory if needed). When c is the zero value, the
// file is deleted instead so the "no settings" state matches a fresh
// install.
func saveUserConfig(c userConfig) error {
	p, err := userConfigPath()
	if err != nil {
		return err
	}
	if c == (userConfig{}) {
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove %s: %w", p, err)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(p), err)
	}
	c.Version = 1
	body, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(p, body, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", p, err)
	}
	return nil
}

// runConfigShow prints the current persisted config as JSON, with the
// client secrets replaced by a placeholder: this output ends up in
// terminals, pipes and bug reports.
func runConfigShow(_ context.Context, g *globalOpts, _ []string) error {
	c, err := loadUserConfig()
	if err != nil {
		return err
	}
	c.Prod.ClientSecret = redactSecret(c.Prod.ClientSecret)
	c.Sandbox.ClientSecret = redactSecret(c.Sandbox.ClientSecret)
	return render(g.stdout, c)
}

// redactSecret reports whether a secret is stored without revealing it.
func redactSecret(secret string) string {
	if secret == "" {
		return ""
	}
	return "(set)"
}

// runConfigClientSecret stores (or clears) the OAuth client secret for
// the selected environment, which is what enables refreshable logins.
//
//	geni config client-secret <secret>        # store it
//	geni config client-secret ""              # clear it
//	geni -sandbox config client-secret <s>    # same, for sandbox
func runConfigClientSecret(_ context.Context, g *globalOpts, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: geni config client-secret <secret|\"\">")
	}

	c, err := loadUserConfig()
	if err != nil {
		return err
	}

	app := &c.Prod
	if g.sandbox {
		app = &c.Sandbox
	}
	app.ClientSecret = args[0]

	if err := saveUserConfig(c); err != nil {
		return err
	}
	if args[0] == "" {
		_, _ = fmt.Fprintln(g.stderr, "client secret cleared; logins fall back to the client-side flow")
	} else {
		_, _ = fmt.Fprintln(g.stderr, "client secret stored; run \"geni login\" to get a refreshable token")
	}
	return nil
}

// runConfigClientID stores (or clears) the OAuth client id, for callers
// running against their own registered Geni application.
//
//	geni config client-id <id>
//	geni config client-id ""
func runConfigClientID(_ context.Context, g *globalOpts, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: geni config client-id <id|\"\">")
	}

	c, err := loadUserConfig()
	if err != nil {
		return err
	}

	app := &c.Prod
	if g.sandbox {
		app = &c.Sandbox
	}
	app.ClientID = args[0]

	if err := saveUserConfig(c); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(g.stderr, "client id set to %q\n", args[0])
	return nil
}

// runConfigBrowser sets (or clears) the persisted browser preference.
//
//	geni config browser <name>   # store the browser to use
//	geni config browser ""       # clear the stored value
func runConfigBrowser(_ context.Context, g *globalOpts, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: geni config browser <name|\"\">")
	}
	name := args[0]
	if name != "" && !slices.Contains(browsercookies.SupportedBrowsers, name) {
		return fmt.Errorf("invalid browser %q (supported: %v)", name, browsercookies.SupportedBrowsers)
	}
	c, err := loadUserConfig()
	if err != nil {
		return err
	}
	c.Browser = name
	if err := saveUserConfig(c); err != nil {
		return err
	}
	if name == "" {
		_, _ = fmt.Fprintln(g.stderr, "browser preference cleared")
	} else {
		_, _ = fmt.Fprintf(g.stderr, "browser set to %s\n", name)
	}
	return nil
}
