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

// runConfigShow prints the current persisted config as JSON.
func runConfigShow(_ context.Context, g *globalOpts, _ []string) error {
	c, err := loadUserConfig()
	if err != nil {
		return err
	}
	return render(g.stdout, c)
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
