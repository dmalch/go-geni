// Package transport owns the cross-cutting HTTP layer for go-geni:
// access-token injection, rate limiting (with dynamic re-tuning from
// X-API-Rate-* response headers), retry on transient errors / 429 /
// 401, error sentinels for 403 (ErrAccessDenied) and 404
// (ErrResourceNotFound), and bulk-read coalescing via the Coalescer
// interface (see BulkCoalescer for the generic implementation).
//
// Resource packages in this module take *transport.Client as a
// dependency and call its Do method; they never wire HTTP plumbing
// themselves. The top-level geni package exposes a façade that
// constructs one *transport.Client and shares it across resource
// clients.
package transport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	"golang.org/x/oauth2"
	"golang.org/x/time/rate"
)

// Client is the shared HTTP transport used by every resource client.
type Client struct {
	tokenSource   oauth2.TokenSource
	useSandboxEnv bool
	client        *http.Client
	limiter       *rate.Limiter
	urlMap        *sync.Map
}

// New constructs a Client. useSandboxEnv toggles between
// sandbox.geni.com and www.geni.com. The rate limiter starts at 1 rps
// and is re-tuned dynamically from each response's X-API-Rate-*
// headers.
func New(tokenSource oauth2.TokenSource, useSandboxEnv bool) *Client {
	return &Client{
		tokenSource:   tokenSource,
		useSandboxEnv: useSandboxEnv,
		client:        &http.Client{},
		limiter:       rate.NewLimiter(rate.Every(1*time.Second), 1),
		urlMap:        &sync.Map{},
	}
}

// BaseURL returns the prod or sandbox HTTP host (with trailing slash)
// configured at construction time. Resource packages call this to
// build endpoint URLs.
func (c *Client) BaseURL() string { return BaseURL(c.useSandboxEnv) }

// APIURL returns the prod or sandbox API host (with "api/" suffix and
// trailing slash). Used when stripping URL prefixes from response
// bodies — e.g. ProfileResponse.Unions.
func (c *Client) APIURL() string { return APIURL(c.useSandboxEnv) }

// UseSandbox reports whether the client targets sandbox.geni.com.
func (c *Client) UseSandbox() bool { return c.useSandboxEnv }

// SetHTTPClient replaces the inner *http.Client. Intended for tests
// that want to inject a fake http.RoundTripper.
func (c *Client) SetHTTPClient(h *http.Client) { c.client = h }

// SetLimiter replaces the rate limiter. Intended for tests that need
// to control timing to exercise coalescing.
func (c *Client) SetLimiter(l *rate.Limiter) { c.limiter = l }

// Limiter returns the rate limiter. Intended for tests that need to
// pre-consume tokens to force concurrent calls to queue together.
func (c *Client) Limiter() *rate.Limiter { return c.limiter }

// Response is the result of a request issued through DoWithResponse:
// the decoded body together with the HTTP response headers. Most
// callers only need the body and use Do; DoWithResponse exists for
// the rare endpoint whose contract carries data in a header — e.g.
// /user/add returns the new account's OAuth token in the
// X-API-OAuth-access_token header.
type Response struct {
	Body   []byte
	Header http.Header
}

// Do sends req and returns the response body. It handles auth header
// / query-param injection, rate-limit pacing (with dynamic re-tune),
// retry on transient errors / 429 / 401, and error sentinel
// translation for 403 (ErrAccessDenied) and 404 (ErrResourceNotFound).
//
// If a Coalescer is supplied (nil is fine), in-flight reads of the
// same resource type collapse into a single bulk request — see
// BulkCoalescer.
func (c *Client) Do(ctx context.Context, req *http.Request, coalescer Coalescer) ([]byte, error) {
	resp, err := c.do(ctx, req, coalescer)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// DoWithResponse behaves like Do — same auth injection, rate-limit
// pacing, retry, and error-sentinel translation — but returns the
// full Response, headers included, and does not support bulk-read
// coalescing. Use it for endpoints whose contract puts data in a
// response header.
func (c *Client) DoWithResponse(ctx context.Context, req *http.Request) (*Response, error) {
	return c.do(ctx, req, nil)
}

func (c *Client) do(ctx context.Context, req *http.Request, coalescer Coalescer) (*Response, error) {
	if err := c.addStandardHeadersAndQueryParams(req); err != nil {
		return nil, err
	}

	return retry.DoWithData(
		func() (*Response, error) {
			limiterCtx, limiterCtxCancelFunc := context.WithCancel(ctx)
			defer limiterCtxCancelFunc()

			if coalescer != nil {
				c.urlMap.Store(coalescer.RequestKey(), limiterCtxCancelFunc)
			}

			if err := c.limiter.Wait(limiterCtx); err != nil {
				if !errors.Is(err, context.Canceled) {
					slog.Error("Error waiting for rate limiter", "error", err)
					return nil, err
				}
			}

			if coalescer != nil {
				if cachedRes, ok := c.urlMap.LoadAndDelete(coalescer.RequestKey()); ok && cachedRes != nil {
					if res, ok := cachedRes.([]byte); ok {
						slog.Debug("Using cached response")
						return &Response{Body: res}, nil
					}
				}
				coalescer.PrepareBulkRequest(req, c.urlMap)
			}

			slog.Debug("Sending request", "method", req.Method, "url", redactURL(req.URL))
			res, err := c.client.Do(req)
			if err != nil {
				return nil, translateTransportError(err)
			}
			defer func(Body io.ReadCloser) {
				_ = Body.Close()
			}(res.Body)

			body, err := io.ReadAll(res.Body)
			if err != nil {
				slog.Error("Error reading response", "error", err)
				return nil, err
			}

			apiRateWindow := res.Header.Get("X-API-Rate-Window")
			apiRateLimit := res.Header.Get("X-API-Rate-Limit")

			slog.Debug("Received response", "status", res.StatusCode, "X-API-Rate-Window", apiRateWindow, "X-API-Rate-Limit", apiRateLimit)
			slog.Debug("Received response body", "status", res.StatusCode, "body", string(body), "X-API-Rate-Window", apiRateWindow, "X-API-Rate-Limit", apiRateLimit)

			secondsUntilRetry, err := strconv.Atoi(apiRateWindow)
			if err == nil {
				if apiRateLimitNumber, err := strconv.Atoi(apiRateLimit); err == nil {
					newLimit := rate.Every(time.Duration(secondsUntilRetry+5) * time.Second / time.Duration(apiRateLimitNumber))
					if c.limiter.Limit() != newLimit {
						slog.Debug("Setting rate limit", "limit", newLimit, "seconds_until_retry", secondsUntilRetry, "api_rate_limit", apiRateLimit)
						c.limiter.SetLimit(newLimit)
					}
					if c.limiter.Burst() != apiRateLimitNumber {
						slog.Debug("Setting rate burst", "burst", apiRateLimitNumber)
						c.limiter.SetBurst(apiRateLimitNumber)
					}
				}
			}

			if res.StatusCode != http.StatusOK {
				return nil, translateStatusError(res.StatusCode, secondsUntilRetry, body)
			}

			if coalescer != nil {
				parsed, err := coalescer.ParseBulkResponse(req, body, c.urlMap)
				if err != nil {
					return nil, err
				}
				return &Response{Body: parsed, Header: res.Header}, nil
			}
			return &Response{Body: body, Header: res.Header}, nil
		},
		retry.RetryIf(func(err error) bool {
			var er errRetry
			return errors.As(err, &er)
		}),
		retry.Context(ctx),
		retry.Attempts(4),
		retry.Delay(2*time.Second),
		retry.MaxJitter(2*time.Second),
		retry.DelayType(retry.CombineDelay(retry.FixedDelay, retry.RandomDelay)),
		retry.OnRetry(func(n uint, err error) {
			slog.Debug("Retrying request", "attempt", n+1, "error", err)
		}),
	)
}

// translateTransportError maps transient network failures (DNS not
// found, broken pipe, connection reset, timeouts) to errRetry so the
// retry-go RetryIf hook picks them up. All other transport errors
// propagate unchanged.
func translateTransportError(err error) error {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		slog.Error("DNS lookup failed", "error", err)
		if dnsErr.IsNotFound {
			return newErrRetry(500, 1)
		}
	}

	var netOpErr *net.OpError
	if errors.As(err, &netOpErr) {
		lowerErr := strings.ToLower(netOpErr.Error())
		if strings.Contains(lowerErr, "broken pipe") {
			slog.Error("Broken pipe error", "error", err)
			return newErrRetry(500, 1)
		}
		if strings.Contains(lowerErr, "connection reset by peer") {
			slog.Error("Connection reset by peer error", "error", err)
			return newErrRetry(500, 1)
		}
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		slog.Error("Network timeout error", "error", err)
		return newErrRetry(504, 1)
	}

	slog.Error("Error sending request", "error", err)
	return err
}

// translateStatusError maps non-200 statuses to the public error
// sentinels (ErrAccessDenied / ErrResourceNotFound), to errRetry for
// retryable codes (429/401), or to Incapsula / generic errors.
func translateStatusError(statusCode, secondsUntilRetry int, body []byte) error {
	switch statusCode {
	case http.StatusTooManyRequests:
		slog.Warn("Received 429 Too Many Requests, retrying...", "X-API-Rate-Window", secondsUntilRetry)
		return newErrRetry(statusCode, secondsUntilRetry)
	case http.StatusUnauthorized:
		slog.Warn("Received 401 Unauthorized, retrying...")
		return newErrRetry(statusCode, 1)
	case http.StatusForbidden:
		slog.Warn("Received 403 Forbidden.")
		return ErrAccessDenied
	case http.StatusNotFound:
		slog.Warn("Received 404 Not Found.")
		return ErrResourceNotFound
	}

	if strings.Contains(string(body), "Request unsuccessful. Incapsula incident ID:") {
		// Incapsula is a DDoS protection service that Geni uses. If we
		// get a response with this message, it means that the request
		// was blocked by Incapsula.
		slog.Warn("Incapsula blocked request.")
		return fmt.Errorf("incapsula blocked request")
	}

	slog.Error("Non-OK HTTP status", "status", statusCode, "body", string(body))
	return fmt.Errorf("non-OK HTTP status: %d, body: %s", statusCode, string(body))
}
