package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

func TestClientID(t *testing.T) {
	RegisterTestingT(t)
	Expect(clientID(false)).To(Equal("1855"))
	Expect(clientID(true)).To(Equal("8"))
}

func TestTokenCacheFilePath(t *testing.T) {
	t.Run("production path", func(t *testing.T) {
		RegisterTestingT(t)
		home := t.TempDir()
		t.Setenv("HOME", home)

		p, err := tokenCacheFilePath(false)
		Expect(err).ToNot(HaveOccurred())
		Expect(p).To(Equal(filepath.Join(home, ".genealogy", "geni_token.json")))
	})

	t.Run("sandbox path", func(t *testing.T) {
		RegisterTestingT(t)
		home := t.TempDir()
		t.Setenv("HOME", home)

		p, err := tokenCacheFilePath(true)
		Expect(err).ToNot(HaveOccurred())
		Expect(p).To(Equal(filepath.Join(home, ".genealogy", "geni_sandbox_token.json")))
	})
}

// clearAuthEnv isolates a test from whatever the developer exported.
func clearAuthEnv(t *testing.T) {
	t.Helper()

	t.Setenv("HOME", t.TempDir())
	for _, name := range []string{
		"GENI_ACCESS_TOKEN",
		"GENI_CLIENT_ID", "GENI_CLIENT_SECRET",
		"GENI_SANDBOX_CLIENT_ID", "GENI_SANDBOX_CLIENT_SECRET",
	} {
		t.Setenv(name, "")
	}
}

func TestClientApp(t *testing.T) {
	t.Run("Falls back to the built-in application with no secret", func(t *testing.T) {
		RegisterTestingT(t)
		clearAuthEnv(t)

		app, err := clientApp(false)
		Expect(err).ToNot(HaveOccurred())
		Expect(app.ClientID).To(Equal("1855"))
		Expect(app.ClientSecret).To(BeEmpty())
	})

	t.Run("Reads the environment first", func(t *testing.T) {
		RegisterTestingT(t)
		clearAuthEnv(t)
		Expect(saveUserConfig(userConfig{Prod: oauthApp{ClientID: "cfg", ClientSecret: "cfg-secret"}})).To(Succeed())
		t.Setenv("GENI_CLIENT_ID", "env")
		t.Setenv("GENI_CLIENT_SECRET", "env-secret")

		app, err := clientApp(false)
		Expect(err).ToNot(HaveOccurred())
		Expect(app.ClientID).To(Equal("env"))
		Expect(app.ClientSecret).To(Equal("env-secret"))
	})

	// A secret alone is enough: the built-in id stays in place, which is
	// what the owner of the built-in application needs.
	t.Run("Keeps the built-in id when only a secret is exported", func(t *testing.T) {
		RegisterTestingT(t)
		clearAuthEnv(t)
		t.Setenv("GENI_CLIENT_SECRET", "env-secret")

		app, err := clientApp(false)
		Expect(err).ToNot(HaveOccurred())
		Expect(app.ClientID).To(Equal("1855"))
		Expect(app.ClientSecret).To(Equal("env-secret"))
	})

	t.Run("Reads the stored config when the environment is empty", func(t *testing.T) {
		RegisterTestingT(t)
		clearAuthEnv(t)
		Expect(saveUserConfig(userConfig{Prod: oauthApp{ClientID: "cfg", ClientSecret: "cfg-secret"}})).To(Succeed())

		app, err := clientApp(false)
		Expect(err).ToNot(HaveOccurred())
		Expect(app.ClientID).To(Equal("cfg"))
		Expect(app.ClientSecret).To(Equal("cfg-secret"))
	})

	t.Run("Keeps the environments apart", func(t *testing.T) {
		RegisterTestingT(t)
		clearAuthEnv(t)
		Expect(saveUserConfig(userConfig{
			Prod:    oauthApp{ClientSecret: "prod-secret"},
			Sandbox: oauthApp{ClientSecret: "sandbox-secret"},
		})).To(Succeed())

		prod, err := clientApp(false)
		Expect(err).ToNot(HaveOccurred())
		Expect(prod.ClientSecret).To(Equal("prod-secret"))

		sandbox, err := clientApp(true)
		Expect(err).ToNot(HaveOccurred())
		Expect(sandbox.ClientID).To(Equal("8"))
		Expect(sandbox.ClientSecret).To(Equal("sandbox-secret"))
	})
}

// The client secret is the only thing standing between a daily browser
// round trip and a token that renews itself, so the selection has to be
// unambiguous.
func TestAuthParts(t *testing.T) {
	t.Run("Selects the client-side flow when no secret is configured", func(t *testing.T) {
		RegisterTestingT(t)
		clearAuthEnv(t)

		_, refresher, interactive, err := authParts(false)
		Expect(err).ToNot(HaveOccurred())
		Expect(refresher).To(BeNil())
		Expect(interactive).ToNot(BeNil())
	})

	t.Run("Selects the refreshable server-side flow when a secret is configured", func(t *testing.T) {
		RegisterTestingT(t)
		clearAuthEnv(t)
		t.Setenv("GENI_CLIENT_SECRET", "app-secret")

		cachePath, refresher, interactive, err := authParts(false)
		Expect(err).ToNot(HaveOccurred())
		Expect(refresher).ToNot(BeNil())
		Expect(interactive).ToNot(BeNil())
		Expect(cachePath).To(HaveSuffix("geni_token.json"))
	})
}

// Every case here must fail before a browser could possibly open.
func TestRunLoginFlagValidation(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"rejects a port above the range", []string{"login", "-port", "70000"}, "invalid -port"},
		{"rejects a negative port", []string{"login", "-port", "-2"}, "invalid -port"},
		{"rejects a non-numeric port", []string{"login", "-port", "abc"}, "invalid value"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)
			clearAuthEnv(t)

			var out, errb bytes.Buffer
			code := run(t.Context(), tc.args, strings.NewReader(""), &out, &errb)

			Expect(code).To(Equal(1))
			Expect(errb.String()).To(ContainSubstring(tc.want))
		})
	}

	// The guard has to survive the flag parsing that now precedes it.
	t.Run("still refuses to run when GENI_ACCESS_TOKEN is set", func(t *testing.T) {
		RegisterTestingT(t)
		clearAuthEnv(t)
		t.Setenv("GENI_ACCESS_TOKEN", "static-token")

		var out, errb bytes.Buffer
		code := run(t.Context(), []string{"login"}, strings.NewReader(""), &out, &errb)

		Expect(code).To(Equal(1))
		Expect(errb.String()).To(ContainSubstring("unset it"))
	})
}

// The stored config ends up in terminals, pipes and bug reports.
func TestRunConfigShowRedactsSecrets(t *testing.T) {
	RegisterTestingT(t)
	clearAuthEnv(t)
	Expect(saveUserConfig(userConfig{
		Prod:    oauthApp{ClientID: "1855", ClientSecret: "prod-secret"},
		Sandbox: oauthApp{ClientSecret: "sandbox-secret"},
	})).To(Succeed())

	var out, errb bytes.Buffer
	code := run(t.Context(), []string{"config", "show"}, strings.NewReader(""), &out, &errb)

	Expect(code).To(Equal(0))
	Expect(out.String()).ToNot(ContainSubstring("prod-secret"))
	Expect(out.String()).ToNot(ContainSubstring("sandbox-secret"))
	Expect(out.String()).To(ContainSubstring("(set)"))
	Expect(out.String()).To(ContainSubstring("1855"))
}

func TestRunTokenStatus(t *testing.T) {
	t.Run("Reports a logged-out environment", func(t *testing.T) {
		RegisterTestingT(t)
		clearAuthEnv(t)

		var out, errb bytes.Buffer
		code := run(t.Context(), []string{"token", "status"}, strings.NewReader(""), &out, &errb)

		Expect(code).To(Equal(0))
		Expect(out.String()).To(ContainSubstring(`"logged_in": false`))
		Expect(out.String()).To(ContainSubstring(`"refreshable": false`))
	})

	t.Run("Never prints the token itself", func(t *testing.T) {
		RegisterTestingT(t)
		clearAuthEnv(t)
		t.Setenv("GENI_CLIENT_SECRET", "app-secret")

		home := os.Getenv("HOME")
		path := filepath.Join(home, ".genealogy", "geni_token.json")
		Expect(os.MkdirAll(filepath.Dir(path), 0o700)).To(Succeed())
		Expect(os.WriteFile(path, []byte(
			`{"access_token":"super-secret","refresh_token":"rt","expiry":"2000-01-01T00:00:00Z"}`), 0o600)).To(Succeed())

		var out, errb bytes.Buffer
		code := run(t.Context(), []string{"token", "status"}, strings.NewReader(""), &out, &errb)

		Expect(code).To(Equal(0))
		Expect(out.String()).ToNot(ContainSubstring("super-secret"))
		Expect(out.String()).ToNot(ContainSubstring(`"rt"`))
		Expect(out.String()).To(ContainSubstring(`"has_refresh_token": true`))
		Expect(out.String()).To(ContainSubstring(`"expired": true`))
		Expect(out.String()).To(ContainSubstring(`"refreshable": true`))
	})
}

func TestRunLogout(t *testing.T) {
	t.Run("removes an existing cache file", func(t *testing.T) {
		RegisterTestingT(t)
		home := t.TempDir()
		t.Setenv("HOME", home)
		path := filepath.Join(home, ".genealogy", "geni_sandbox_token.json")
		Expect(os.MkdirAll(filepath.Dir(path), 0o755)).To(Succeed())
		Expect(os.WriteFile(path, []byte(`{"access_token":"x"}`), 0o600)).To(Succeed())

		g := &globalOpts{sandbox: true}
		Expect(runLogout(context.Background(), g, nil)).To(Succeed())

		_, statErr := os.Stat(path)
		Expect(os.IsNotExist(statErr)).To(BeTrue())
	})

	t.Run("is a no-op when the cache file is absent", func(t *testing.T) {
		RegisterTestingT(t)
		home := t.TempDir()
		t.Setenv("HOME", home)

		g := &globalOpts{sandbox: true}
		Expect(runLogout(context.Background(), g, nil)).To(Succeed())
	})
}
