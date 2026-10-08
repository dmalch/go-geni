package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"

	"github.com/dmalch/go-geni/transport"
	"github.com/dmalch/go-geni/web"
)

const apiUsage = "usage: geni api [-X method] [-f key=value]... [-F key=value]... [-H 'Name: value']... " +
	"[-input file] [-i] [-paginate] [-web] <endpoint>"

// newAPITransport builds the transport behind "geni api". Indirected so
// tests can point it at a fake server.
var newAPITransport = func(g *globalOpts) (*transport.Client, error) {
	ts, err := authChain(g.sandbox)
	if err != nil {
		return nil, err
	}
	return transport.New(ts, g.sandbox), nil
}

// newAPIWebClient builds the cookie-authenticated client behind
// "geni api -web", after the same one-time consent every AJAX command
// asks for. Indirected so tests can point it at a fake server.
var newAPIWebClient = func(g *globalOpts) (*web.Client, error) {
	if err := ensureWebConsent(g); err != nil {
		return nil, err
	}
	cookies, err := loadWebCookies(g)
	if err != nil {
		return nil, err
	}
	return newWebClient(g, cookies)
}

// apiOpts holds the parsed "geni api" flags.
type apiOpts struct {
	method   string
	fields   []rawField
	headers  repeatableFlag
	input    string
	include  bool
	paginate bool
	web      bool
}

// apiCall is the request the flags describe, before either mode turns
// it into an *http.Request for its own host and authentication.
type apiCall struct {
	method string
	fields []apiField
	body   []byte // from -input; nil when absent
	header http.Header
}

// runAPI handles "geni api [flags] <endpoint>" — a raw call to any Geni
// API endpoint in the spirit of "gh api", authenticated, rate limited and
// retried like every other command. With -web it calls a geni.com page
// or AJAX endpoint with the browser session instead.
//
// Fields default the method to POST. On a GET, or when -input supplies
// the body, they go in the query string; otherwise they form a JSON
// object for the API and a form for -web. The response body is printed
// as is, re-indented when it is JSON; a status that is not a success
// prints the body too and exits 1.
func runAPI(ctx context.Context, g *globalOpts, args []string) error {
	fs := flag.NewFlagSet("geni api", flag.ContinueOnError)
	fs.SetOutput(g.stderr)
	// The long names are gh's, so "geni api" takes the same muscle memory.
	var o apiOpts
	fs.StringVar(&o.method, "X", "", "HTTP method (default GET, or POST when fields or -input are given)")
	fs.StringVar(&o.method, "method", "", "same as -X")
	fs.Var(fieldFlag{&o.fields, false}, "f", "add a string parameter key=value (repeatable)")
	fs.Var(fieldFlag{&o.fields, false}, "raw-field", "same as -f")
	fs.Var(fieldFlag{&o.fields, true}, "F",
		"add a typed parameter key=value: true, false, null and integers become JSON, @file reads a file, @- stdin (repeatable)")
	fs.Var(fieldFlag{&o.fields, true}, "field", "same as -F")
	fs.Var(&o.headers, "H", "add a request header 'Name: value' (repeatable)")
	fs.Var(&o.headers, "header", "same as -H")
	fs.StringVar(&o.input, "input", "", "read the request body from a file (- for stdin)")
	fs.BoolVar(&o.include, "i", false, "print the status line and response headers before the body")
	fs.BoolVar(&o.include, "include", false, "same as -i")
	fs.BoolVar(&o.paginate, "paginate", false, "follow next_page and print every page (API GETs only)")
	fs.BoolVar(&o.web, "web", false, "call a geni.com web path with the browser session instead of the OAuth API")

	positional, err := parseInterleaved(fs, args)
	if err != nil {
		return err
	}
	if len(positional) != 1 {
		return errors.New(apiUsage)
	}

	call, err := o.call(g.stdin)
	if err != nil {
		return err
	}
	if o.paginate && (o.web || call.method != http.MethodGet) {
		return errors.New("-paginate follows the API's next_page links, so it needs an API GET")
	}

	if o.web {
		return runAPIWeb(ctx, g, o, positional[0], call)
	}
	return runAPIOAuth(ctx, g, o, positional[0], call)
}

// call resolves the flags into the request to send.
func (o apiOpts) call(stdin io.Reader) (apiCall, error) {
	var c apiCall
	if o.input == "-" && slices.ContainsFunc(o.fields, func(f rawField) bool {
		return f.typed && strings.HasSuffix(f.arg, "=@-")
	}) {
		return c, errors.New("stdin can only be read once: -input - and -F key=@- both need it")
	}

	fields, err := parseAPIFields(o.fields, stdin)
	if err != nil {
		return c, err
	}
	c.fields = fields

	if o.input != "" {
		if o.input == "-" {
			c.body, err = io.ReadAll(stdin)
		} else {
			c.body, err = os.ReadFile(o.input)
		}
		if err != nil {
			return c, fmt.Errorf("read -input: %w", err)
		}
		if c.body == nil {
			c.body = []byte{}
		}
	}

	c.header = http.Header{}
	for _, h := range o.headers {
		name, value, ok := strings.Cut(h, ":")
		if !ok || strings.TrimSpace(name) == "" {
			return c, fmt.Errorf("header %q must be 'Name: value'", h)
		}
		c.header.Add(strings.TrimSpace(name), strings.TrimSpace(value))
	}

	c.method = strings.ToUpper(o.method)
	if c.method == "" {
		c.method = http.MethodGet
		if len(c.fields) > 0 || c.body != nil {
			c.method = http.MethodPost
		}
	}
	return c, nil
}

// fieldsInQuery reports whether the fields belong in the query string
// rather than the body: they do on a GET, and whenever -input already
// supplies the body.
func (c apiCall) fieldsInQuery() bool {
	return c.method == http.MethodGet || c.method == http.MethodHead || c.body != nil
}

func runAPIOAuth(ctx context.Context, g *globalOpts, o apiOpts, endpoint string, call apiCall) error {
	tc, err := newAPITransport(g)
	if err != nil {
		return err
	}
	u, err := resolveAPIURL(endpoint, tc.UseSandbox())
	if err != nil {
		return err
	}

	body := call.body
	switch {
	case call.fieldsInQuery():
		addQuery(u, apiFieldValues(call.fields))
	case len(call.fields) > 0:
		obj, err := apiFieldsJSON(call.fields)
		if err != nil {
			return err
		}
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(obj); err != nil {
			return err
		}
		body = bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
	}
	// Geni mishandles raw UTF-8 in a request body; a JSON one can carry
	// every rune as a \uXXXX escape instead.
	if json.Valid(body) {
		body = []byte(transport.EscapeStringToUTF(string(body)))
	}

	for {
		var reader io.Reader
		if body != nil {
			reader = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, call.method, u.String(), reader)
		if err != nil {
			return err
		}
		copyHeader(req.Header, call.header)

		resp, err := tc.DoRaw(ctx, req)
		if err != nil {
			return err
		}
		if err := writeAPIResponse(g.stdout, resp.StatusCode, resp.Header, resp.Body, o.include); err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			return httpStatusError(resp.StatusCode)
		}
		if !o.paginate {
			return nil
		}

		next := nextPageURL(resp.Body)
		if next == "" {
			return nil
		}
		nextURL, err := resolveAPIURL(next, tc.UseSandbox())
		if err != nil {
			return fmt.Errorf("next_page: %w", err)
		}
		if nextURL.String() == u.String() {
			return fmt.Errorf("next_page points back at %s", next)
		}
		u = nextURL
	}
}

func runAPIWeb(ctx context.Context, g *globalOpts, o apiOpts, path string, call apiCall) error {
	wc, err := newAPIWebClient(g)
	if err != nil {
		return err
	}
	u, err := resolveWebURL(path, wc.BaseURL())
	if err != nil {
		return err
	}

	// Rails rejects a state-changing request without the session's
	// authenticity_token. The header covers an -input body of any shape;
	// the form field is what geni.com's own forms send.
	var token string
	if call.method != http.MethodGet && call.method != http.MethodHead {
		if token, err = wc.CSRFToken(ctx); err != nil {
			return err
		}
	}

	body := call.body
	contentType := ""
	if call.fieldsInQuery() {
		addQuery(u, apiFieldValues(call.fields))
	} else {
		form := apiFieldValues(call.fields)
		if !form.Has("authenticity_token") {
			form.Set("authenticity_token", token)
		}
		body = []byte(form.Encode())
		contentType = "application/x-www-form-urlencoded"
	}

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, call.method, u.String(), reader)
	if err != nil {
		return err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if token != "" {
		req.Header.Set("X-CSRF-Token", token)
	}
	copyHeader(req.Header, call.header)

	resp, err := wc.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if err := writeAPIResponse(g.stdout, resp.StatusCode, resp.Header, respBody, o.include); err != nil {
		return err
	}
	// geni.com answers a successful form POST with a redirect, and the
	// client does not follow it: say where it pointed.
	if loc := resp.Header.Get("Location"); loc != "" && !o.include {
		_, _ = fmt.Fprintf(g.stderr, "HTTP %d, redirected to %s\n", resp.StatusCode, loc)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return httpStatusError(resp.StatusCode)
	}
	return nil
}

// parseInterleaved parses fs from args, allowing flags after positional
// arguments too ("geni api profile-1 -i"), as gh does. The standard flag
// package stops at the first positional argument.
func parseInterleaved(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		args = fs.Args()
		if len(args) == 0 {
			return positional, nil
		}
		positional = append(positional, args[0])
		args = args[1:]
	}
}

func addQuery(u *url.URL, v url.Values) {
	if len(v) == 0 {
		return
	}
	if u.RawQuery != "" {
		u.RawQuery += "&"
	}
	u.RawQuery += v.Encode()
}

func copyHeader(dst, src http.Header) {
	for name, values := range src {
		dst.Del(name)
		for _, v := range values {
			dst.Add(name, v)
		}
	}
}

// nextPageURL returns the next_page link of a paginated response, or ""
// when there is none.
func nextPageURL(body []byte) string {
	var page struct {
		NextPage string `json:"next_page"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		return ""
	}
	return page.NextPage
}

// writeAPIResponse prints a response: with include, a status line and
// the headers first, as "gh api -i" does; then the body, re-indented
// when it is JSON and byte for byte otherwise.
func writeAPIResponse(w io.Writer, status int, header http.Header, body []byte, include bool) error {
	if include {
		var b strings.Builder
		fmt.Fprintf(&b, "HTTP %d %s\n", status, http.StatusText(status))
		for _, name := range slices.Sorted(maps.Keys(header)) {
			for _, v := range header[name] {
				fmt.Fprintf(&b, "%s: %s\n", name, v)
			}
		}
		b.WriteString("\n")
		if _, err := io.WriteString(w, b.String()); err != nil {
			return err
		}
	}

	var indented bytes.Buffer
	if json.Valid(body) && json.Indent(&indented, body, "", "  ") == nil {
		indented.WriteByte('\n')
		body = indented.Bytes()
	}
	_, err := w.Write(body)
	return err
}

func httpStatusError(status int) error {
	return fmt.Errorf("HTTP %d %s", status, http.StatusText(status))
}
