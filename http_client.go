package geni

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	"golang.org/x/oauth2"
	"golang.org/x/time/rate"
	"log/slog"
)

var ErrResourceNotFound = fmt.Errorf("resource not found")
var ErrAccessDenied = fmt.Errorf("access denied")

type errCode429WithRetry struct {
	statusCode        int
	secondsUntilRetry int
}

func (e errCode429WithRetry) Error() string {
	return fmt.Sprintf("received %d status, retry in %d seconds", e.statusCode, e.secondsUntilRetry)
}

func newErrWithRetry(statusCode int, secondsUntilRetry int) error {
	return errCode429WithRetry{
		statusCode:        statusCode,
		secondsUntilRetry: secondsUntilRetry,
	}
}

type Client struct {
	useSandboxEnv bool
	tokenSource   oauth2.TokenSource
	client        *http.Client
	limiter       *rate.Limiter
	urlMap        *sync.Map
}

func NewClient(tokenSource oauth2.TokenSource, useSandboxEnv bool) *Client {
	return &Client{
		useSandboxEnv: useSandboxEnv,
		tokenSource:   tokenSource,
		client:        &http.Client{},
		limiter:       rate.NewLimiter(rate.Every(1*time.Second), 1),
		urlMap:        &sync.Map{},
	}
}

func BaseURL(useSandboxEnv bool) string {
	if useSandboxEnv {
		return geniSandboxUrl
	}
	return geniProdUrl
}

func apiUrl(useSandboxEnv bool) string {
	if useSandboxEnv {
		return geniSandboxApiUrl
	}
	return geniProdApiUrl
}

type opt struct {
	getRequestKey          func() string
	prepareBulkRequestFrom func(*http.Request, *sync.Map)
	parseBulkResponse      func(*http.Request, []byte, *sync.Map) ([]byte, error)
}

func withRequestKey(fn func() string) func(*opt) {
	return func(o *opt) {
		o.getRequestKey = fn
	}
}

func withPrepareBulkRequest(fn func(*http.Request, *sync.Map)) func(*opt) {
	return func(o *opt) {
		o.prepareBulkRequestFrom = fn
	}
}

func withParseBulkResponse(fn func(*http.Request, []byte, *sync.Map) ([]byte, error)) func(*opt) {
	return func(o *opt) {
		o.parseBulkResponse = fn
	}
}

func (c *Client) doRequest(ctx context.Context, req *http.Request, opts ...func(*opt)) ([]byte, error) {
	// Initialize the opt struct with default no-op functions
	options := opt{
		prepareBulkRequestFrom: func(*http.Request, *sync.Map) {},
	}

	// Apply the provided opts to the options struct
	for _, o := range opts {
		o(&options)
	}

	if err := c.addStandardHeadersAndQueryParams(req); err != nil {
		return nil, err
	}

	// Retry logic using retry-go
	return retry.DoWithData(
		func() ([]byte, error) {
			limiterCtx, limiterCtxCancelFunc := context.WithCancel(ctx)
			defer limiterCtxCancelFunc()

			if options.getRequestKey != nil {
				// Store the key in the map
				c.urlMap.Store(options.getRequestKey(), limiterCtxCancelFunc)
			}

			if err := c.limiter.Wait(limiterCtx); err != nil {
				// If the context is canceled, we should not return an error
				if !errors.Is(err, context.Canceled) {
					slog.Error("Error waiting for rate limiter", "error", err)
					return nil, err
				}
			}

			// Check if the response is already cached
			if options.getRequestKey != nil {
				if cachedRes, ok := c.urlMap.LoadAndDelete(options.getRequestKey()); ok && cachedRes != nil {
					if res, ok := cachedRes.([]byte); ok {
						slog.Debug("Using cached response")
						return res, nil
					}
				}

				options.prepareBulkRequestFrom(req, c.urlMap)
			}

			slog.Debug("Sending request", "method", req.Method, "url", redactURL(req.URL))
			res, err := c.client.Do(req)
			if err != nil {
				var dnsErr *net.DNSError
				if errors.As(err, &dnsErr) {
					slog.Error("DNS lookup failed", "error", err)
					if dnsErr.IsNotFound {
						return nil, newErrWithRetry(500, 1)
					}
				}

				var netOpErr *net.OpError
				if errors.As(err, &netOpErr) {
					lowerErr := strings.ToLower(netOpErr.Error())
					if strings.Contains(lowerErr, "broken pipe") {
						slog.Error("Broken pipe error", "error", err)
						return nil, newErrWithRetry(500, 1)
					}
					if strings.Contains(lowerErr, "connection reset by peer") {
						slog.Error("Connection reset by peer error", "error", err)
						return nil, newErrWithRetry(500, 1)
					}
				}

				var netErr net.Error
				if errors.As(err, &netErr) && netErr.Timeout() {
					slog.Error("Network timeout error", "error", err)
					return nil, newErrWithRetry(504, 1)
				}

				slog.Error("Error sending request", "error", err)
				return nil, err
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
				if res.StatusCode == http.StatusTooManyRequests {
					slog.Warn("Received 429 Too Many Requests, retrying...", "X-API-Rate-Window", secondsUntilRetry)
					return nil, newErrWithRetry(res.StatusCode, secondsUntilRetry)
				}

				if res.StatusCode == http.StatusUnauthorized {
					slog.Warn("Received 401 Unauthorized, retrying...")
					return nil, newErrWithRetry(res.StatusCode, 1)
				}

				if res.StatusCode == http.StatusForbidden {
					slog.Warn("Received 403 Forbidden.")
					return nil, ErrAccessDenied
				}

				if res.StatusCode == http.StatusNotFound {
					slog.Warn("Received 404 Not Found.")
					return nil, ErrResourceNotFound
				}

				if strings.Contains(string(body), "Request unsuccessful. Incapsula incident ID:") {
					// Incapsula is a DDoS protection service that Geni uses. If we get a response
					// with this message, it means that the request was blocked by Incapsula.
					slog.Warn("Incapsula blocked request.")
					return nil, fmt.Errorf("incapsula blocked request")
				}

				slog.Error("Non-OK HTTP status", "status", res.StatusCode, "body", string(body))
				return nil, fmt.Errorf("non-OK HTTP status: %s, body: %s", res.Status, string(body))
			}

			if options.parseBulkResponse != nil {
				return options.parseBulkResponse(req, body, c.urlMap)
			}

			return body, nil
		},
		retry.RetryIf(func(err error) bool {
			var errCode429WithRetry errCode429WithRetry
			return errors.As(err, &errCode429WithRetry)
		}),
		retry.Context(ctx),
		retry.Attempts(4),
		retry.Delay(2*time.Second),     // Wait 2 seconds between retries
		retry.MaxJitter(2*time.Second), // Add up to 2 seconds of jitter to each retry
		retry.DelayType(retry.CombineDelay(retry.FixedDelay, retry.RandomDelay)),
		retry.OnRetry(func(n uint, err error) {
			slog.Debug("Retrying request", "attempt", n+1, "error", err)
		}),
	)
}

func redactURL(u *url.URL) string {
	redacted := *u
	q := redacted.Query()
	if q.Has("access_token") {
		q.Set("access_token", "REDACTED")
		redacted.RawQuery = q.Encode()
	}
	return redacted.String()
}

func (c *Client) addStandardHeadersAndQueryParams(req *http.Request) error {
	query := req.URL.Query()

	token, err := c.tokenSource.Token()
	if err != nil {
		return fmt.Errorf("error getting token: %w", err)
	}

	query.Add("access_token", token.AccessToken)
	query.Add("api_version", apiVersion)
	// The returned data structures will contain urls to other objects by default,
	// unless the request includes 'only_ids=true.' Passing only_ids will force the
	// system to return ids only.
	query.Add("only_ids", "true")

	req.URL.RawQuery = query.Encode()
	req.Header.Add("Accept", "application/json")
	// Only inject application/json when the caller hasn't already set
	// a Content-Type — multipart upload endpoints (e.g. photo/add)
	// pre-set their own header with the boundary parameter and would
	// otherwise end up with two conflicting Content-Type values.
	if req.Header.Get("Content-Type") == "" {
		req.Header.Add("Content-Type", "application/json")
	}
	req.Header.Add("User-Agent", "terraform-provider-genealogy/0.1")

	return nil
}
