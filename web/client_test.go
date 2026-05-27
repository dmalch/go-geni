package web

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

func TestCookiesFromHeader(t *testing.T) {
	t.Run("parses semicolon-separated cookie header", func(t *testing.T) {
		RegisterTestingT(t)
		cookies := CookiesFromHeader("_geni_session=abc; remember_user_token=xyz")

		Expect(cookies).To(HaveLen(2))
		names := []string{cookies[0].Name, cookies[1].Name}
		Expect(names).To(ContainElements("_geni_session", "remember_user_token"))
	})

	t.Run("empty header yields no cookies", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(CookiesFromHeader("")).To(BeEmpty())
	})
}

func TestNewClient_RequiresCookies(t *testing.T) {
	RegisterTestingT(t)
	_, err := NewClient(Options{})
	Expect(err).To(MatchError(ErrNoCookies))
}

func TestClient_Do_SendsCookiesAndUserAgent(t *testing.T) {
	RegisterTestingT(t)
	var captured *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Clone(r.Context())
		_, _ = io.WriteString(w, "ok")
	}))
	defer srv.Close()

	c, err := NewClient(Options{
		Cookies:   CookiesFromHeader("_geni_session=abc"),
		UserAgent: "go-geni/test",
		BaseURL:   srv.URL,
	})
	Expect(err).ToNot(HaveOccurred())

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/", nil)
	resp, err := c.Do(req)
	Expect(err).ToNot(HaveOccurred())
	_ = resp.Body.Close()

	Expect(captured.Header.Get("User-Agent")).To(Equal("go-geni/test"))
	cookie, err := captured.Cookie("_geni_session")
	Expect(err).ToNot(HaveOccurred())
	Expect(cookie.Value).To(Equal("abc"))
}

func TestClient_Do_DetectsLoginRedirect(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/login?next=/x", http.StatusFound)
	}))
	defer srv.Close()

	c, _ := NewClient(Options{Cookies: CookiesFromHeader("_geni_session=abc"), BaseURL: srv.URL})

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/anything", nil)
	resp, err := c.Do(req)
	if resp != nil {
		_ = resp.Body.Close()
	}

	Expect(errors.Is(err, ErrNotLoggedIn)).To(BeTrue(), "expected ErrNotLoggedIn, got %v", err)
}

func TestClient_Do_DetectsIncapsulaBlock(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Iinfo", "1-2-3")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `<html><head><title>Request unsuccessful. Incapsula incident ID: 123</title></head></html>`)
	}))
	defer srv.Close()

	c, _ := NewClient(Options{Cookies: CookiesFromHeader("_geni_session=abc"), BaseURL: srv.URL})

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/anything", nil)
	resp, err := c.Do(req)
	if resp != nil {
		_ = resp.Body.Close()
	}

	Expect(errors.Is(err, ErrBlocked)).To(BeTrue(), "expected ErrBlocked, got %v", err)
}

func TestClient_Do_RespectsRateLimit(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	defer srv.Close()

	c, _ := NewClient(Options{
		Cookies:   CookiesFromHeader("_geni_session=abc"),
		BaseURL:   srv.URL,
		RateLimit: 5, // 5 rps → ~200ms between calls
	})

	start := time.Now()
	for range 3 {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/", nil)
		resp, err := c.Do(req)
		Expect(err).ToNot(HaveOccurred())
		_ = resp.Body.Close()
	}
	elapsed := time.Since(start)

	// 3 requests at 5 rps means at minimum ~400ms (2 limiter waits).
	Expect(elapsed).To(BeNumerically(">=", 300*time.Millisecond))
}

func TestClient_CSRFToken_CachesAcrossCalls(t *testing.T) {
	RegisterTestingT(t)
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = io.WriteString(w, `<form><input type="hidden" name="authenticity_token" value="cached-token"></form>`)
	}))
	defer srv.Close()

	c, _ := NewClient(Options{
		Cookies:        CookiesFromHeader("_geni_session=abc"),
		BaseURL:        srv.URL,
		CSRFSourcePath: "/csrf",
	})

	a, err := c.CSRFToken(context.Background())
	Expect(err).ToNot(HaveOccurred())
	Expect(a).To(Equal("cached-token"))

	b, err := c.CSRFToken(context.Background())
	Expect(err).ToNot(HaveOccurred())
	Expect(b).To(Equal("cached-token"))

	Expect(hits).To(Equal(1), "second CSRFToken call should hit the cache, not the server")
}

func TestClient_CSRFToken_RefreshesAfterInvalidate(t *testing.T) {
	RegisterTestingT(t)
	tokens := []string{"first", "second"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok := tokens[0]
		tokens = tokens[1:]
		_, _ = io.WriteString(w, `<form><input type="hidden" name="authenticity_token" value="`+tok+`"></form>`)
	}))
	defer srv.Close()

	c, _ := NewClient(Options{
		Cookies:        CookiesFromHeader("_geni_session=abc"),
		BaseURL:        srv.URL,
		CSRFSourcePath: "/csrf",
	})

	a, _ := c.CSRFToken(context.Background())
	Expect(a).To(Equal("first"))

	c.InvalidateCSRF()

	b, _ := c.CSRFToken(context.Background())
	Expect(b).To(Equal("second"))
}

func TestClient_BaseURL_DefaultsToGeniHost(t *testing.T) {
	RegisterTestingT(t)
	c, err := NewClient(Options{Cookies: CookiesFromHeader("_geni_session=abc")})
	Expect(err).ToNot(HaveOccurred())
	Expect(c.BaseURL()).To(Equal("https://www.geni.com"))
}

func TestClient_BaseURL_TrimsTrailingSlash(t *testing.T) {
	RegisterTestingT(t)
	c, err := NewClient(Options{
		Cookies: CookiesFromHeader("_geni_session=abc"),
		BaseURL: "https://example.test/",
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(c.BaseURL()).To(Equal("https://example.test"))
}
