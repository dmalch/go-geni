package tree

import (
	"net/http"
	"strconv"
)

// Option customises an outgoing request for the family-graph and
// path-to endpoints. Options only set the parameters they understand;
// passing an option to a method that doesn't honor it is harmless.
type Option func(*http.Request)

// WithGenerations sets the generations query parameter on
// [Client.Ancestors]. Values ≤0 are a no-op; values >20 are clamped
// to 20 (the Geni-documented maximum).
func WithGenerations(n int) Option {
	return func(r *http.Request) {
		if n <= 0 {
			return
		}
		if n > 20 {
			n = 20
		}
		setQueryParam(r, "generations", strconv.Itoa(n))
	}
}

// WithPathType sets the path_type query parameter on [Client.PathTo].
// An empty value is a no-op (Geni defaults to "closest").
func WithPathType(t PathType) Option {
	return func(r *http.Request) {
		if t == "" {
			return
		}
		setQueryParam(r, "path_type", string(t))
	}
}

// WithRefresh forces a recomputation of a path-to result. The flag is
// only emitted when v is true.
func WithRefresh(v bool) Option {
	return boolOption("refresh", v, true)
}

// WithSearch toggles the path-to search behavior. Geni defaults to
// true, so the parameter is only emitted when v is false (i.e. to
// opt out).
func WithSearch(v bool) Option {
	return boolOption("search", v, false)
}

// WithSkipEmail suppresses the email notification path-to would
// otherwise send. Only emitted when v is true.
func WithSkipEmail(v bool) Option {
	return boolOption("skip_email", v, true)
}

// WithSkipNotify suppresses the on-site notification path-to would
// otherwise send. Only emitted when v is true.
func WithSkipNotify(v bool) Option {
	return boolOption("skip_notify", v, true)
}

// boolOption emits "<name>=<v>" only when v equals emitWhen — every
// caller passes the polarity Geni's server-side default does not
// already cover, so we never send no-op parameters.
func boolOption(name string, v, emitWhen bool) Option {
	return func(r *http.Request) {
		if v != emitWhen {
			return
		}
		setQueryParam(r, name, strconv.FormatBool(v))
	}
}

func setQueryParam(r *http.Request, key, value string) {
	q := r.URL.Query()
	q.Set(key, value)
	r.URL.RawQuery = q.Encode()
}
