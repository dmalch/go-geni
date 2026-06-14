package transport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"golang.org/x/net/http2"
	"golang.org/x/oauth2"
)

// headerEchoTransport is a fake http.RoundTripper that returns a
// fixed status, body, and headers — used to drive DoWithResponse
// without a live server.
type headerEchoTransport struct {
	status int
	body   string
	header http.Header
}

func (t *headerEchoTransport) RoundTrip(*http.Request) (*http.Response, error) {
	h := t.header
	if h == nil {
		h = make(http.Header)
	}
	return &http.Response{
		StatusCode: t.status,
		Body:       io.NopCloser(strings.NewReader(t.body)),
		Header:     h,
	}, nil
}

func newClientWith(rt http.RoundTripper) *Client {
	c := New(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"}), true)
	c.SetHTTPClient(&http.Client{Transport: rt})
	return c
}

func TestDoWithResponse(t *testing.T) {
	t.Run("returns the body and the response headers", func(t *testing.T) {
		RegisterTestingT(t)
		header := http.Header{}
		header.Set("X-API-OAuth-access_token", "new-token")
		c := newClientWith(&headerEchoTransport{
			status: http.StatusOK,
			body:   `{"ok":true}`,
			header: header,
		})

		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://www.geni.com/api/user/add", nil)
		resp, err := c.DoWithResponse(context.Background(), req)

		Expect(err).ToNot(HaveOccurred())
		Expect(string(resp.Body)).To(Equal(`{"ok":true}`))
		Expect(resp.Header.Get("X-API-OAuth-access_token")).To(Equal("new-token"))
	})

	t.Run("maps a 403 to ErrAccessDenied", func(t *testing.T) {
		RegisterTestingT(t)
		c := newClientWith(&headerEchoTransport{status: http.StatusForbidden})

		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://www.geni.com/api/user/add", nil)
		_, err := c.DoWithResponse(context.Background(), req)

		Expect(err).To(MatchError(ErrAccessDenied))
	})
}

func TestRedactURL(t *testing.T) {
	t.Run("Redacts access_token from URL", func(t *testing.T) {
		RegisterTestingT(t)
		u, _ := url.Parse("https://www.geni.com/api/profile?access_token=secret123&fields=id")

		result := redactURL(u)

		Expect(result).To(ContainSubstring("access_token=REDACTED"))
		Expect(result).ToNot(ContainSubstring("secret123"))
		Expect(result).To(ContainSubstring("fields=id"))
	})

	t.Run("Returns URL unchanged when no access_token", func(t *testing.T) {
		RegisterTestingT(t)
		u, _ := url.Parse("https://www.geni.com/api/profile?fields=id")

		result := redactURL(u)

		Expect(result).To(Equal("https://www.geni.com/api/profile?fields=id"))
	})

	t.Run("Preserves other query params", func(t *testing.T) {
		RegisterTestingT(t)
		u, _ := url.Parse("https://www.geni.com/api/profile?access_token=secret&api_version=1&only_ids=true")

		result := redactURL(u)

		Expect(result).To(ContainSubstring("access_token=REDACTED"))
		Expect(result).To(ContainSubstring("api_version=1"))
		Expect(result).To(ContainSubstring("only_ids=true"))
	})
}

func TestErrRetry(t *testing.T) {
	t.Run("Formats error message correctly", func(t *testing.T) {
		RegisterTestingT(t)
		err := errRetry{statusCode: 429, secondsUntilRetry: 30}
		Expect(err.Error()).To(Equal("received 429 status, retry in 30 seconds"))
	})

	t.Run("Works with different status codes", func(t *testing.T) {
		RegisterTestingT(t)
		err := errRetry{statusCode: 401, secondsUntilRetry: 1}
		Expect(err.Error()).To(Equal("received 401 status, retry in 1 seconds"))
	})
}

func TestTranslateTransportError(t *testing.T) {
	t.Run("HTTP/2 stream error with CANCEL is retryable", func(t *testing.T) {
		RegisterTestingT(t)
		streamErr := http2.StreamError{StreamID: 333, Code: http2.ErrCodeCancel}

		got := translateTransportError(streamErr)

		var er errRetry
		Expect(errors.As(got, &er)).To(BeTrue(), "expected errRetry, got %T: %v", got, got)
	})

	t.Run("HTTP/2 stream error with REFUSED_STREAM is retryable", func(t *testing.T) {
		RegisterTestingT(t)
		streamErr := http2.StreamError{StreamID: 7, Code: http2.ErrCodeRefusedStream}

		got := translateTransportError(streamErr)

		var er errRetry
		Expect(errors.As(got, &er)).To(BeTrue(), "expected errRetry, got %T: %v", got, got)
	})

	t.Run("HTTP/2 stream error wrapped in url.Error is retryable", func(t *testing.T) {
		RegisterTestingT(t)
		// http.Client.Do wraps transport errors in *url.Error before returning them.
		wrapped := &url.Error{Op: "Post", URL: "https://www.geni.com/api/profile/add", Err: http2.StreamError{StreamID: 333, Code: http2.ErrCodeCancel}}

		got := translateTransportError(wrapped)

		var er errRetry
		Expect(errors.As(got, &er)).To(BeTrue(), "expected errRetry, got %T: %v", got, got)
	})

	t.Run("Unrelated error propagates unchanged", func(t *testing.T) {
		RegisterTestingT(t)
		orig := fmt.Errorf("unexpected boom")

		got := translateTransportError(orig)

		var er errRetry
		Expect(errors.As(got, &er)).To(BeFalse())
		Expect(got).To(MatchError(orig))
	})
}

func TestTranslateStatusError(t *testing.T) {
	t.Run("Transient server error is retryable", func(t *testing.T) {
		RegisterTestingT(t)
		for _, statusCode := range []int{
			http.StatusBadGateway,         // 502 — Geni "Will Be Right Back!" page
			http.StatusServiceUnavailable, // 503
			http.StatusGatewayTimeout,     // 504
		} {
			got := translateStatusError(statusCode, 1, []byte("<h1>Geni will be right back!</h1>"))

			var er errRetry
			Expect(errors.As(got, &er)).To(BeTrue(), "expected errRetry for %d, got %T: %v", statusCode, got, got)
		}
	})

	t.Run("Rate-limit and auth stay retryable", func(t *testing.T) {
		RegisterTestingT(t)
		for _, statusCode := range []int{http.StatusTooManyRequests, http.StatusUnauthorized} {
			got := translateStatusError(statusCode, 1, nil)

			var er errRetry
			Expect(errors.As(got, &er)).To(BeTrue(), "expected errRetry for %d, got %T: %v", statusCode, got, got)
		}
	})

	t.Run("Client errors map to sentinels", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(translateStatusError(http.StatusForbidden, 0, nil)).To(MatchError(ErrAccessDenied))
		Expect(translateStatusError(http.StatusNotFound, 0, nil)).To(MatchError(ErrResourceNotFound))
	})

	t.Run("Oversized error body is summarized", func(t *testing.T) {
		RegisterTestingT(t)
		body := []byte("<!DOCTYPE html>" + strings.Repeat("x", 4000) + "</html>")

		got := translateStatusError(http.StatusInternalServerError, 0, body)

		var er errRetry
		Expect(errors.As(got, &er)).To(BeFalse(), "500 should not be retryable")
		Expect(got.Error()).To(ContainSubstring("… (truncated)"))
		Expect(len(got.Error())).To(BeNumerically("<", len(body)))
	})

	t.Run("Incapsula block is still detected", func(t *testing.T) {
		RegisterTestingT(t)
		body := []byte("Request unsuccessful. Incapsula incident ID: 1234-5678")

		got := translateStatusError(http.StatusInternalServerError, 0, body)

		Expect(got).To(MatchError(ContainSubstring("incapsula blocked request")))
	})
}

func TestEscapeStringToUTF(t *testing.T) {
	t.Run("ASCII characters pass through unchanged", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(EscapeStringToUTF("Hello World")).To(Equal("Hello World"))
	})

	t.Run("Non-ASCII characters are escaped", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(EscapeStringToUTF("café")).To(Equal("caf\\u00e9"))
	})

	t.Run("Empty string returns empty", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(EscapeStringToUTF("")).To(Equal(""))
	})

	t.Run("Mixed ASCII and non-ASCII content", func(t *testing.T) {
		RegisterTestingT(t)
		result := EscapeStringToUTF("Hello Wörld")
		Expect(result).To(Equal("Hello W\\u00f6rld"))
	})

	t.Run("Cyrillic characters are escaped", func(t *testing.T) {
		RegisterTestingT(t)
		result := EscapeStringToUTF("Привет")
		Expect(result).To(Equal("\\u041f\\u0440\\u0438\\u0432\\u0435\\u0442"))
	})
}
