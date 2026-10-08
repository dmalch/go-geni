package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	. "github.com/onsi/gomega"
	"golang.org/x/oauth2"
	"golang.org/x/time/rate"

	"github.com/dmalch/go-geni/transport"
	"github.com/dmalch/go-geni/web"
)

// apiRequest is what the fake server saw of one request.
type apiRequest struct {
	method string
	path   string
	query  url.Values
	header http.Header
	body   string
}

// apiServer is an httptest server that records every request and answers
// each with respond.
type apiServer struct {
	*httptest.Server
	mu       sync.Mutex
	requests []apiRequest
}

func newAPIServer(t *testing.T, respond func(w http.ResponseWriter, r *http.Request)) *apiServer {
	t.Helper()
	s := &apiServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		s.requests = append(s.requests, apiRequest{
			method: r.Method, path: r.URL.Path, query: r.URL.Query(), header: r.Header.Clone(), body: string(body),
		})
		s.mu.Unlock()
		respond(w, r)
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *apiServer) seen() []apiRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]apiRequest(nil), s.requests...)
}

// redirectTransport sends every request to target whatever its host, the
// way the real transport's fixed geni.com URLs can reach a test server.
type redirectTransport struct{ target *url.URL }

func (rt redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	r.URL.Scheme = rt.target.Scheme
	r.URL.Host = rt.target.Host
	return http.DefaultTransport.RoundTrip(r)
}

// useAPIServer points "geni api" at s with a static token and no rate
// limit.
func useAPIServer(t *testing.T, s *apiServer) {
	t.Helper()
	target, err := url.Parse(s.URL)
	Expect(err).ToNot(HaveOccurred())
	orig := newAPITransport
	newAPITransport = func(g *globalOpts) (*transport.Client, error) {
		c := transport.New(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"}), g.sandbox)
		c.SetHTTPClient(&http.Client{Transport: redirectTransport{target}})
		c.SetLimiter(rate.NewLimiter(rate.Inf, 1))
		return c, nil
	}
	t.Cleanup(func() { newAPITransport = orig })
}

// useWebServer points "geni api -web" at s with a fixed session cookie.
func useWebServer(t *testing.T, s *apiServer) {
	t.Helper()
	orig := newAPIWebClient
	newAPIWebClient = func(*globalOpts) (*web.Client, error) {
		return web.NewClient(web.Options{
			Cookies:   []*http.Cookie{{Name: "_geni_session", Value: "sess"}},
			BaseURL:   s.URL,
			RateLimit: 1000,
		})
	}
	t.Cleanup(func() { newAPIWebClient = orig })
}

// runCLI runs the geni command line with args, isolated from the
// developer's own environment.
func runCLI(t *testing.T, stdin string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	clearAuthEnv(t)
	t.Setenv("GENI_USE_SANDBOX", "")
	var out, errb bytes.Buffer
	code = run(t.Context(), args, strings.NewReader(stdin), &out, &errb)
	return code, out.String(), errb.String()
}

func respondJSON(body string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	}
}

func TestAPI(t *testing.T) {
	t.Run("a GET sends fields as the query and prints indented JSON", func(t *testing.T) {
		RegisterTestingT(t)
		s := newAPIServer(t, respondJSON(`{"id":"profile-1","name":"Ivan"}`))
		useAPIServer(t, s)

		code, stdout, stderr := runCLI(t, "", "api", "-X", "GET", "profile/search", "-f", "names=Ivan Petrov")

		Expect(stderr).To(BeEmpty())
		Expect(code).To(Equal(0))
		Expect(stdout).To(Equal("{\n  \"id\": \"profile-1\",\n  \"name\": \"Ivan\"\n}\n"))
		req := s.seen()[0]
		Expect(req.method).To(Equal(http.MethodGet))
		Expect(req.path).To(Equal("/api/profile/search"))
		Expect(req.query["names"]).To(Equal([]string{"Ivan Petrov"}))
		Expect(req.query["access_token"]).To(Equal([]string{"test-token"}))
		Expect(req.query["only_ids"]).To(Equal([]string{"true"}))
		Expect(req.body).To(BeEmpty())
	})

	t.Run("fields make it a POST with a JSON body, non-ASCII escaped", func(t *testing.T) {
		RegisterTestingT(t)
		s := newAPIServer(t, respondJSON(`{}`))
		useAPIServer(t, s)

		code, _, stderr := runCLI(t, "", "api", "profile-1/update",
			"-f", "first_name=Иван", "-F", "birth[date][year]=1900", "-F", "is_alive=false")

		Expect(stderr).To(BeEmpty())
		Expect(code).To(Equal(0))
		req := s.seen()[0]
		Expect(req.method).To(Equal(http.MethodPost))
		Expect(req.path).To(Equal("/api/profile-1/update"))
		Expect(req.header.Get("Content-Type")).To(Equal("application/json"))
		// Geni mishandles raw UTF-8 in a request body.
		Expect(req.body).To(ContainSubstring(transport.EscapeStringToUTF("Иван")))
		Expect(req.body).ToNot(ContainSubstring("Иван"))
		var sent map[string]any
		Expect(json.Unmarshal([]byte(req.body), &sent)).To(Succeed())
		Expect(sent).To(Equal(map[string]any{
			"first_name": "Иван",
			"birth":      map[string]any{"date": map[string]any{"year": float64(1900)}},
			"is_alive":   false,
		}))
	})

	t.Run("-input - sends stdin as the body and moves fields to the query", func(t *testing.T) {
		RegisterTestingT(t)
		s := newAPIServer(t, respondJSON(`{}`))
		useAPIServer(t, s)

		code, _, stderr := runCLI(t, `{"about_me":"Тверь"}`, "api", "-input", "-", "profile-1/update", "-f", "fields=id")

		Expect(stderr).To(BeEmpty())
		Expect(code).To(Equal(0))
		req := s.seen()[0]
		Expect(req.method).To(Equal(http.MethodPost))
		Expect(req.body).To(Equal(`{"about_me":"` + transport.EscapeStringToUTF("Тверь") + `"}`))
		Expect(req.query["fields"]).To(Equal([]string{"id"}))
	})

	t.Run("flags may follow the endpoint, and -H reaches the server", func(t *testing.T) {
		RegisterTestingT(t)
		s := newAPIServer(t, respondJSON(`{}`))
		useAPIServer(t, s)

		code, _, _ := runCLI(t, "", "api", "profile-1", "-H", "X-Test: yes", "-method", "GET")

		Expect(code).To(Equal(0))
		Expect(s.seen()[0].header.Get("X-Test")).To(Equal("yes"))
	})

	t.Run("-i prints the status line and the headers first", func(t *testing.T) {
		RegisterTestingT(t)
		s := newAPIServer(t, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("X-API-Rate-Limit", "40")
			_, _ = io.WriteString(w, `{"ok":true}`)
		})
		useAPIServer(t, s)

		code, stdout, _ := runCLI(t, "", "api", "-i", "user")

		Expect(code).To(Equal(0))
		Expect(stdout).To(HavePrefix("HTTP 200 OK\n"))
		Expect(stdout).To(ContainSubstring("\nX-Api-Rate-Limit: 40\n"))
		Expect(stdout).To(HaveSuffix("\n\n{\n  \"ok\": true\n}\n"))
	})

	t.Run("an error status prints the body and exits 1", func(t *testing.T) {
		RegisterTestingT(t)
		s := newAPIServer(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"error":{"message":"Profile not found"}}`)
		})
		useAPIServer(t, s)

		code, stdout, stderr := runCLI(t, "", "api", "profile-0")

		Expect(code).To(Equal(1))
		Expect(stdout).To(ContainSubstring(`"message": "Profile not found"`))
		Expect(stderr).To(Equal("geni api: HTTP 404 Not Found\n"))
	})

	t.Run("-paginate follows next_page and prints every page", func(t *testing.T) {
		RegisterTestingT(t)
		s := newAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("page") == "2" {
				_, _ = io.WriteString(w, `{"results":[{"id":"profile-2"}],"page":2}`)
				return
			}
			// Geni's links name the public host and may carry the old token.
			_, _ = fmt.Fprint(w, `{"results":[{"id":"profile-1"}],"page":1,`+
				`"next_page":"https://www.geni.com/api/profile/search?names=x&page=2&access_token=stale"}`)
		})
		useAPIServer(t, s)

		code, stdout, stderr := runCLI(t, "", "api", "-paginate", "profile/search?names=x")

		Expect(stderr).To(BeEmpty())
		Expect(code).To(Equal(0))
		reqs := s.seen()
		Expect(reqs).To(HaveLen(2))
		Expect(reqs[1].query["page"]).To(Equal([]string{"2"}))
		Expect(reqs[1].query["access_token"]).To(Equal([]string{"test-token"}))
		Expect(reqs[1].query["only_ids"]).To(Equal([]string{"true"}))

		dec := json.NewDecoder(strings.NewReader(stdout))
		var pages []map[string]any
		for dec.More() {
			var p map[string]any
			Expect(dec.Decode(&p)).To(Succeed())
			pages = append(pages, p)
		}
		Expect(pages).To(HaveLen(2))
		Expect(pages[1]["page"]).To(Equal(float64(2)))
	})

	t.Run("a URL on another host is refused before any request", func(t *testing.T) {
		RegisterTestingT(t)
		s := newAPIServer(t, respondJSON(`{}`))
		useAPIServer(t, s)

		code, _, stderr := runCLI(t, "", "api", "https://example.com/api/profile-1")

		Expect(code).To(Equal(1))
		Expect(stderr).To(ContainSubstring("not a production API URL"))
		Expect(s.seen()).To(BeEmpty())
	})

	t.Run("-sandbox targets the sandbox", func(t *testing.T) {
		RegisterTestingT(t)
		s := newAPIServer(t, respondJSON(`{}`))
		useAPIServer(t, s)

		code, _, stderr := runCLI(t, "", "-sandbox", "api", "https://www.geni.com/api/user")

		Expect(code).To(Equal(1))
		Expect(stderr).To(ContainSubstring("drop -sandbox"))
		Expect(s.seen()).To(BeEmpty())
	})

	t.Run("usage errors", func(t *testing.T) {
		for _, args := range [][]string{
			{"api"},
			{"api", "profile-1", "profile-2"},
			{"api", "-paginate", "-X", "POST", "profile-1"},
			{"api", "-paginate", "-web", "/list/data_conflicts"},
			{"api", "-input", "-", "-F", "about=@-", "profile-1/update"},
			{"api", "-H", "no-colon", "profile-1"},
		} {
			t.Run(strings.Join(args, " "), func(t *testing.T) {
				RegisterTestingT(t)
				s := newAPIServer(t, respondJSON(`{}`))
				useAPIServer(t, s)
				useWebServer(t, s)

				code, _, stderr := runCLI(t, "", args...)

				Expect(code).To(Equal(1))
				Expect(stderr).ToNot(BeEmpty())
				Expect(s.seen()).To(BeEmpty())
			})
		}
	})
}

func TestAPIWeb(t *testing.T) {
	t.Run("a GET sends the session cookie and prints HTML as is", func(t *testing.T) {
		RegisterTestingT(t)
		s := newAPIServer(t, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "<html>conflicts</html>")
		})
		useWebServer(t, s)

		code, stdout, stderr := runCLI(t, "", "api", "-web", "-X", "GET", "/list/data_conflicts", "-f", "page=2")

		Expect(stderr).To(BeEmpty())
		Expect(code).To(Equal(0))
		Expect(stdout).To(Equal("<html>conflicts</html>"))
		req := s.seen()[0]
		Expect(req.path).To(Equal("/list/data_conflicts"))
		Expect(req.query["page"]).To(Equal([]string{"2"}))
		Expect(req.query.Has("access_token")).To(BeFalse())
		Expect(req.header.Get("Cookie")).To(ContainSubstring("_geni_session=sess"))
	})

	t.Run("a POST sends a form with the CSRF token, and a redirect is success", func(t *testing.T) {
		RegisterTestingT(t)
		s := newAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				_, _ = io.WriteString(w, `<form><input name="authenticity_token" value="tok123"></form>`)
				return
			}
			w.Header().Set("Location", "/people/Ivan/1")
			w.WriteHeader(http.StatusFound)
		})
		useWebServer(t, s)

		code, _, stderr := runCLI(t, "", "api", "-web", "-X", "POST", "/merge/resolve/abc",
			"-f", "resolve[name]=__unchanged__")

		Expect(code).To(Equal(0))
		Expect(stderr).To(ContainSubstring("/people/Ivan/1"))
		reqs := s.seen()
		Expect(reqs).To(HaveLen(2))
		post := reqs[1]
		Expect(post.path).To(Equal("/merge/resolve/abc"))
		Expect(post.header.Get("Content-Type")).To(Equal("application/x-www-form-urlencoded"))
		Expect(post.header.Get("X-CSRF-Token")).To(Equal("tok123"))
		form, err := url.ParseQuery(post.body)
		Expect(err).ToNot(HaveOccurred())
		Expect(form).To(Equal(url.Values{
			"resolve[name]":      {"__unchanged__"},
			"authenticity_token": {"tok123"},
		}))
	})

	t.Run("a redirect to /login means the session is gone", func(t *testing.T) {
		RegisterTestingT(t)
		s := newAPIServer(t, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Location", "/login")
			w.WriteHeader(http.StatusFound)
		})
		useWebServer(t, s)

		code, _, stderr := runCLI(t, "", "api", "-web", "/list/data_conflicts")

		Expect(code).To(Equal(1))
		Expect(stderr).To(ContainSubstring(web.ErrNotLoggedIn.Error()))
	})

	t.Run("an error status prints the body and exits 1", func(t *testing.T) {
		RegisterTestingT(t)
		s := newAPIServer(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = io.WriteString(w, "bad token")
		})
		useWebServer(t, s)

		code, stdout, stderr := runCLI(t, "", "api", "-web", "/x")

		Expect(code).To(Equal(1))
		Expect(stdout).To(Equal("bad token"))
		Expect(stderr).To(Equal("geni api: HTTP 422 Unprocessable Entity\n"))
	})
}
