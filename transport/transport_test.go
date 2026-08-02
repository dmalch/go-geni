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
	"time"

	"github.com/avast/retry-go/v4"
	. "github.com/onsi/gomega"
	"golang.org/x/net/http2"
	"golang.org/x/oauth2"
	"golang.org/x/time/rate"
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

// scriptedTransport returns each queued response in turn, then repeats the
// last one. Used to drive the retry ladder without a live server.
type scriptedTransport struct {
	responses []scriptedResponse
	calls     int
}

type scriptedResponse struct {
	status int
	body   string
}

func (t *scriptedTransport) RoundTrip(*http.Request) (*http.Response, error) {
	r := t.responses[min(t.calls, len(t.responses)-1)]
	t.calls++
	return &http.Response{
		StatusCode: r.status,
		Body:       io.NopCloser(strings.NewReader(r.body)),
		Header:     make(http.Header),
	}, nil
}

const incapsulaBody = "Request unsuccessful. Incapsula incident ID: 1234-5678"

// fastIncapsulaRetries shortens the 45s block delay so the retry ladder can be
// exercised in milliseconds, and restores it afterwards.
func fastIncapsulaRetries(t *testing.T) {
	t.Helper()
	original := incapsulaRetryDelay
	incapsulaRetryDelay = time.Millisecond
	t.Cleanup(func() { incapsulaRetryDelay = original })
}

func TestIncapsulaIsRetryable(t *testing.T) {
	t.Run("recovers when a block clears", func(t *testing.T) {
		RegisterTestingT(t)
		fastIncapsulaRetries(t)
		rt := &scriptedTransport{responses: []scriptedResponse{
			{http.StatusInternalServerError, incapsulaBody},
			{http.StatusInternalServerError, incapsulaBody},
			{http.StatusOK, `{"ok":true}`},
		}}
		c := newClientWith(rt)
		c.SetLimiter(rate.NewLimiter(rate.Inf, 1))

		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://www.geni.com/api/profile-1", nil)
		body, err := c.Do(context.Background(), req, nil)

		// Before this change the FIRST block failed the whole operation.
		Expect(err).ToNot(HaveOccurred())
		Expect(string(body)).To(Equal(`{"ok":true}`))
		Expect(rt.calls).To(Equal(3))
	})

	t.Run("gives up after maxIncapsulaRetries, keeping the message", func(t *testing.T) {
		RegisterTestingT(t)
		fastIncapsulaRetries(t)
		rt := &scriptedTransport{responses: []scriptedResponse{
			{http.StatusInternalServerError, incapsulaBody},
		}}
		c := newClientWith(rt)
		c.SetLimiter(rate.NewLimiter(rate.Inf, 1))

		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://www.geni.com/api/profile-1", nil)
		_, err := c.Do(context.Background(), req, nil)

		// The message is load-bearing: geni-tree-terraform's apply_converge
		// classifies transient vs permanent failures by matching on it.
		Expect(err).To(MatchError(ContainSubstring("incapsula blocked request")))
		// One initial attempt plus maxIncapsulaRetries — never the full ladder.
		Expect(rt.calls).To(Equal(1 + maxIncapsulaRetries))
	})
}

func TestRetryDelay(t *testing.T) {
	t.Run("an Incapsula block waits far longer than the normal ladder", func(t *testing.T) {
		RegisterTestingT(t)
		got := retryDelay(1, errIncapsula{}, &retry.Config{})

		Expect(got).To(Equal(incapsulaRetryDelay))
		Expect(got).To(BeNumerically(">", 10*time.Second))
	})

	t.Run("every other error keeps the previous behaviour", func(t *testing.T) {
		RegisterTestingT(t)
		// The delay is FixedDelay+RandomDelay, i.e. base..base+jitter. Asserting
		// the range rather than an exact value because RandomDelay is random.
		//
		// Note what is deliberately NOT special-cased here: a 429 carrying
		// secondsUntilRetry keeps the same ~2-4s wait it always had. Pacing for
		// those is the shared rate limiter's job — it re-tunes from every
		// response's X-API-Rate-* headers — and this delay must stay out of it.
		cfg := &retry.Config{}
		retry.Delay(2 * time.Second)(cfg)
		retry.MaxJitter(2 * time.Second)(cfg)

		for _, err := range []error{
			errRetry{statusCode: 429, secondsUntilRetry: 30},
			errRetry{statusCode: 401, secondsUntilRetry: 1},
			fmt.Errorf("something else"),
		} {
			got := retryDelay(1, err, cfg)

			Expect(got).To(BeNumerically(">=", 2*time.Second))
			Expect(got).To(BeNumerically("<=", 4*time.Second))
		}
	})
}
