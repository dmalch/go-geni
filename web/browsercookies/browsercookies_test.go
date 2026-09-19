package browsercookies

import (
	"errors"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/steipete/sweetcookie"
)

func TestToHTTPCookies(t *testing.T) {
	t.Run("copies name, value, domain, path, expiry, flags", func(t *testing.T) {
		RegisterTestingT(t)
		exp := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
		src := []sweetcookie.Cookie{{
			Name:     "session",
			Value:    "abc",
			Domain:   "www.geni.com",
			Path:     "/",
			Expires:  &exp,
			HTTPOnly: true,
			Secure:   false,
		}}

		got := toHTTPCookies(src)

		Expect(got).To(HaveLen(1))
		Expect(got[0].Name).To(Equal("session"))
		Expect(got[0].Value).To(Equal("abc"))
		Expect(got[0].Domain).To(Equal("www.geni.com"))
		Expect(got[0].Path).To(Equal("/"))
		Expect(got[0].Expires).To(Equal(exp))
		Expect(got[0].HttpOnly).To(BeTrue())
		Expect(got[0].Secure).To(BeFalse())
	})

	t.Run("nil expiry stays zero on the http.Cookie", func(t *testing.T) {
		RegisterTestingT(t)
		src := []sweetcookie.Cookie{{Name: "x", Value: "y", Expires: nil}}

		got := toHTTPCookies(src)

		Expect(got).To(HaveLen(1))
		Expect(got[0].Expires.IsZero()).To(BeTrue())
	})

	t.Run("nil input → nil output", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(toHTTPCookies(nil)).To(BeNil())
	})
}

func TestFromGeniCom_ReturnsErrNoCookiesWhenEmpty(t *testing.T) {
	RegisterTestingT(t)
	prev := readCookies
	readCookies = func([]sweetcookie.Browser) (sweetcookie.Result, error) {
		return sweetcookie.Result{}, nil
	}
	t.Cleanup(func() { readCookies = prev })

	_, err := FromGeniCom()

	Expect(errors.Is(err, ErrNoCookies)).To(BeTrue(), "expected ErrNoCookies, got %v", err)
}

func TestFromGeniCom_WrapsBackendErrors(t *testing.T) {
	RegisterTestingT(t)
	prev := readCookies
	readCookies = func([]sweetcookie.Browser) (sweetcookie.Result, error) {
		return sweetcookie.Result{}, errors.New("operation not permitted")
	}
	t.Cleanup(func() { readCookies = prev })

	_, err := FromGeniCom()

	Expect(err).To(HaveOccurred())
	Expect(errors.Is(err, ErrFullDiskAccessRequired)).To(BeTrue(),
		"expected ErrFullDiskAccessRequired wrap, got %v", err)
}

func TestFromGeniCom_ForwardsBrowserFilter(t *testing.T) {
	RegisterTestingT(t)
	var got []sweetcookie.Browser
	prev := readCookies
	readCookies = func(bs []sweetcookie.Browser) (sweetcookie.Result, error) {
		got = bs
		return sweetcookie.Result{Cookies: []sweetcookie.Cookie{{Name: "x", Value: "y"}}}, nil
	}
	t.Cleanup(func() { readCookies = prev })

	_, err := FromGeniCom("Safari", "  firefox  ")
	Expect(err).ToNot(HaveOccurred())
	Expect(got).To(Equal([]sweetcookie.Browser{
		sweetcookie.BrowserSafari, sweetcookie.BrowserFirefox,
	}))
}

func TestFromGeniCom_NoArgsPassesNilBrowsers(t *testing.T) {
	RegisterTestingT(t)
	var got []sweetcookie.Browser
	called := false
	prev := readCookies
	readCookies = func(bs []sweetcookie.Browser) (sweetcookie.Result, error) {
		called = true
		got = bs
		return sweetcookie.Result{Cookies: []sweetcookie.Cookie{{Name: "x", Value: "y"}}}, nil
	}
	t.Cleanup(func() { readCookies = prev })

	_, err := FromGeniCom()
	Expect(err).ToNot(HaveOccurred())
	Expect(called).To(BeTrue())
	Expect(got).To(BeNil(), "no args should yield nil so sweetcookie picks the default order")
}

func TestFromGeniCom_RejectsUnknownBrowserBeforeNetwork(t *testing.T) {
	RegisterTestingT(t)
	called := false
	prev := readCookies
	readCookies = func([]sweetcookie.Browser) (sweetcookie.Result, error) {
		called = true
		return sweetcookie.Result{}, nil
	}
	t.Cleanup(func() { readCookies = prev })

	_, err := FromGeniCom("not-a-browser")
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("not-a-browser"))
	Expect(called).To(BeFalse(), "expected validation to short-circuit before readCookies")
}

// sweetcookie reports a store it could not OPEN as a warning and still
// returns (Result{}, nil). Before these cases the caller saw only
// ErrNoCookies — "log in to a browser first" — which is the opposite of
// the truth when the store is full of cookies and merely unreadable.
func TestFromGeniCom_EmptyResultWarnings(t *testing.T) {
	stub := func(t *testing.T, warnings []string) {
		prev := readCookies
		readCookies = func([]sweetcookie.Browser) (sweetcookie.Result, error) {
			return sweetcookie.Result{Warnings: warnings}, nil
		}
		t.Cleanup(func() { readCookies = prev })
	}

	t.Run("Safari permission failure is not a Full-Disk-Access problem", func(t *testing.T) {
		RegisterTestingT(t)
		stub(t, []string{"sweetcookie: Safari read failed: open " +
			"/Users/x/Library/Containers/com.apple.Safari/Data/Library/Cookies/" +
			"Cookies.binarycookies: operation not permitted"})

		_, err := FromGeniCom()

		Expect(err).To(MatchError(ErrSafariCookiesUnreadable))
		Expect(err).ToNot(MatchError(ErrNoCookies), "the store is unreadable, not empty")
		Expect(err.Error()).To(ContainSubstring("GENI_WEB_COOKIES"),
			"the message must name the only fallback that works")
	})

	t.Run("a Chromium store denied by TCC does point at Full Disk Access", func(t *testing.T) {
		RegisterTestingT(t)
		stub(t, []string{"sweetcookie: Chrome read failed: open /Users/x/Library/" +
			"Application Support/Google/Chrome/Default/Cookies: permission denied"})

		_, err := FromGeniCom()

		Expect(err).To(MatchError(ErrFullDiskAccessRequired))
		Expect(err).ToNot(MatchError(ErrSafariCookiesUnreadable))
	})

	t.Run("stores merely absent stay ErrNoCookies but say which were tried", func(t *testing.T) {
		RegisterTestingT(t)
		stub(t, []string{"sweetcookie: Chrome cookie store not found",
			"sweetcookie: Firefox cookie store not found"})

		_, err := FromGeniCom()

		Expect(err).To(MatchError(ErrNoCookies))
		Expect(err.Error()).To(ContainSubstring("Chrome cookie store not found"))
		Expect(err.Error()).To(ContainSubstring("Firefox cookie store not found"))
	})

	t.Run("no warnings at all is still the bare sentinel", func(t *testing.T) {
		RegisterTestingT(t)
		stub(t, nil)

		_, err := FromGeniCom()

		Expect(err).To(MatchError(ErrNoCookies))
	})
}
