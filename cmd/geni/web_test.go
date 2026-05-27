package main

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

// failOnReadReader makes any Read() call fail the test — used to prove
// a code path did NOT prompt.
type failOnReadReader struct{ t *testing.T }

func (f failOnReadReader) Read([]byte) (int, error) {
	f.t.Helper()
	f.t.Fatal("unexpected read from stdin")
	return 0, io.EOF
}

func TestWebConsentFilePath(t *testing.T) {
	RegisterTestingT(t)
	home := t.TempDir()
	t.Setenv("HOME", home)

	p, err := webConsentFilePath()
	Expect(err).ToNot(HaveOccurred())
	Expect(p).To(Equal(filepath.Join(home, ".genealogy", "web_consent.json")))
}

func TestEnsureWebConsent_FirstCallPromptsAndWritesFile(t *testing.T) {
	RegisterTestingT(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GENI_WEB_CONSENT", "")

	stderr := &bytes.Buffer{}
	g := &globalOpts{stdin: strings.NewReader("y\n"), stderr: stderr}

	Expect(ensureWebConsent(g)).To(Succeed())
	Expect(stderr.String()).To(ContainSubstring("undocumented"))
	Expect(stderr.String()).To(ContainSubstring("[y/N]"))

	body, err := os.ReadFile(filepath.Join(home, ".genealogy", "web_consent.json"))
	Expect(err).ToNot(HaveOccurred())
	Expect(string(body)).To(ContainSubstring(`"accepted_at"`))
	Expect(string(body)).To(ContainSubstring(`"version"`))
}

func TestEnsureWebConsent_NoPromptWhenFilePresent(t *testing.T) {
	RegisterTestingT(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GENI_WEB_CONSENT", "")

	path := filepath.Join(home, ".genealogy", "web_consent.json")
	Expect(os.MkdirAll(filepath.Dir(path), 0o755)).To(Succeed())
	Expect(os.WriteFile(path, []byte(`{"accepted_at":"2026-01-01T00:00:00Z","version":1}`), 0o600)).To(Succeed())

	g := &globalOpts{stdin: failOnReadReader{t: t}, stderr: io.Discard}

	Expect(ensureWebConsent(g)).To(Succeed())
}

func TestEnsureWebConsent_EnvVarBypassesPromptAndFileWrite(t *testing.T) {
	RegisterTestingT(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GENI_WEB_CONSENT", "accepted")

	g := &globalOpts{stdin: failOnReadReader{t: t}, stderr: io.Discard}

	Expect(ensureWebConsent(g)).To(Succeed())

	_, err := os.Stat(filepath.Join(home, ".genealogy", "web_consent.json"))
	Expect(os.IsNotExist(err)).To(BeTrue(), "env-var acceptance must not persist a file")
}

func TestEnsureWebConsent_DeclineAborts(t *testing.T) {
	for _, in := range []string{"n\n", "no\n", "\n", ""} {
		t.Run("input="+strings.TrimSpace(in)+"_eol", func(t *testing.T) {
			RegisterTestingT(t)
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("GENI_WEB_CONSENT", "")

			g := &globalOpts{stdin: strings.NewReader(in), stderr: io.Discard}

			Expect(ensureWebConsent(g)).To(HaveOccurred())
			_, err := os.Stat(filepath.Join(home, ".genealogy", "web_consent.json"))
			Expect(os.IsNotExist(err)).To(BeTrue(), "decline must not write the consent file")
		})
	}
}

func TestLoadWebCookies_EnvVarTakesPrecedence(t *testing.T) {
	RegisterTestingT(t)
	t.Setenv("GENI_WEB_COOKIES", "_geni_session=abc; remember_user_token=xyz")

	prev := browserCookieFetcher
	browserCookieFetcher = func(...string) ([]*http.Cookie, error) {
		t.Fatal("browser fallback must not be called when GENI_WEB_COOKIES is set")
		return nil, nil
	}
	t.Cleanup(func() { browserCookieFetcher = prev })

	cookies, err := loadWebCookies(&globalOpts{stderr: io.Discard})

	Expect(err).ToNot(HaveOccurred())
	Expect(cookies).To(HaveLen(2))
	names := []string{cookies[0].Name, cookies[1].Name}
	Expect(names).To(ContainElements("_geni_session", "remember_user_token"))
}

func TestLoadWebCookies_EmptyEnvUsesBrowserFallback(t *testing.T) {
	RegisterTestingT(t)
	t.Setenv("GENI_WEB_COOKIES", "")

	want := []*http.Cookie{{Name: "from-browser", Value: "ok"}}
	var gotBrowsers []string
	prev := browserCookieFetcher
	browserCookieFetcher = func(browsers ...string) ([]*http.Cookie, error) {
		gotBrowsers = browsers
		return want, nil
	}
	t.Cleanup(func() { browserCookieFetcher = prev })

	cookies, err := loadWebCookies(&globalOpts{stderr: io.Discard})

	Expect(err).ToNot(HaveOccurred())
	Expect(cookies).To(Equal(want))
	Expect(gotBrowsers).To(BeEmpty(), "no -browser flag means no browser filter forwarded")
}

func TestLoadWebCookies_ForwardsBrowserFlag(t *testing.T) {
	RegisterTestingT(t)
	t.Setenv("GENI_WEB_COOKIES", "")

	var gotBrowsers []string
	prev := browserCookieFetcher
	browserCookieFetcher = func(browsers ...string) ([]*http.Cookie, error) {
		gotBrowsers = browsers
		return []*http.Cookie{{Name: "x", Value: "y"}}, nil
	}
	t.Cleanup(func() { browserCookieFetcher = prev })

	_, err := loadWebCookies(&globalOpts{stderr: io.Discard, browser: "safari"})
	Expect(err).ToNot(HaveOccurred())
	Expect(gotBrowsers).To(Equal([]string{"safari"}))
}

func TestLoadWebCookies_BrowserFailureWrappedWithHint(t *testing.T) {
	RegisterTestingT(t)
	t.Setenv("GENI_WEB_COOKIES", "")

	boom := errors.New("operation not permitted")
	prev := browserCookieFetcher
	browserCookieFetcher = func(...string) ([]*http.Cookie, error) { return nil, boom }
	t.Cleanup(func() { browserCookieFetcher = prev })

	_, err := loadWebCookies(&globalOpts{stderr: io.Discard})

	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("GENI_WEB_COOKIES"))
}
