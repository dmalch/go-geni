package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

func TestParseAPIFields(t *testing.T) {
	t.Run("-f keeps every value a string", func(t *testing.T) {
		RegisterTestingT(t)
		got, err := parseAPIFields([]rawField{{"a=true", false}, {"b=42", false}, {"c=x=y", false}}, nil)

		Expect(err).ToNot(HaveOccurred())
		Expect(got).To(Equal([]apiField{{"a", "true"}, {"b", "42"}, {"c", "x=y"}}))
	})

	t.Run("-F converts literals and keeps the rest strings", func(t *testing.T) {
		RegisterTestingT(t)
		got, err := parseAPIFields([]rawField{
			{"t=true", true}, {"f=false", true}, {"n=null", true},
			{"i=-42", true}, {"s=Ivan", true}, {"fl=1.5", true},
		}, nil)

		Expect(err).ToNot(HaveOccurred())
		Expect(got).To(Equal([]apiField{
			{"t", true}, {"f", false}, {"n", nil},
			{"i", int64(-42)}, {"s", "Ivan"}, {"fl", "1.5"},
		}))
	})

	t.Run("-F @file reads the file and @- reads stdin", func(t *testing.T) {
		RegisterTestingT(t)
		path := filepath.Join(t.TempDir(), "about.txt")
		Expect(os.WriteFile(path, []byte("born in Tver\n"), 0o600)).To(Succeed())

		got, err := parseAPIFields([]rawField{{"about=@" + path, true}, {"bio=@-", true}},
			strings.NewReader("from stdin"))

		Expect(err).ToNot(HaveOccurred())
		Expect(got).To(Equal([]apiField{{"about", "born in Tver\n"}, {"bio", "from stdin"}}))
	})

	t.Run("stdin can only be read once", func(t *testing.T) {
		RegisterTestingT(t)
		_, err := parseAPIFields([]rawField{{"a=@-", true}, {"b=@-", true}}, strings.NewReader("x"))

		Expect(err).To(MatchError(ContainSubstring("stdin")))
	})

	t.Run("rejects a field without a value or a key", func(t *testing.T) {
		RegisterTestingT(t)
		_, err := parseAPIFields([]rawField{{"names", false}}, nil)
		Expect(err).To(MatchError(ContainSubstring("key=value")))

		_, err = parseAPIFields([]rawField{{"=x", false}}, nil)
		Expect(err).To(MatchError(ContainSubstring("key=value")))
	})
}

func TestAPIFieldsJSON(t *testing.T) {
	t.Run("nests bracketed keys and appends to [] arrays", func(t *testing.T) {
		RegisterTestingT(t)
		got, err := apiFieldsJSON([]apiField{
			{"first_name", "Ivan"},
			{"birth[date][year]", int64(1900)},
			{"birth[date][month]", int64(5)},
			{"nicknames[]", "Vanya"},
			{"nicknames[]", "Vanechka"},
			{"is_alive", false},
		})

		Expect(err).ToNot(HaveOccurred())
		Expect(got).To(Equal(map[string]any{
			"first_name": "Ivan",
			"birth":      map[string]any{"date": map[string]any{"year": int64(1900), "month": int64(5)}},
			"nicknames":  []any{"Vanya", "Vanechka"},
			"is_alive":   false,
		}))
	})

	t.Run("a key cannot be both a value and an object", func(t *testing.T) {
		RegisterTestingT(t)
		_, err := apiFieldsJSON([]apiField{{"birth", "x"}, {"birth[date]", "y"}})
		Expect(err).To(MatchError(ContainSubstring("birth")))
	})

	t.Run("a key cannot be given twice", func(t *testing.T) {
		RegisterTestingT(t)
		_, err := apiFieldsJSON([]apiField{{"a", "1"}, {"a", "2"}})
		Expect(err).To(MatchError(ContainSubstring("more than once")))
	})

	t.Run("rejects malformed keys", func(t *testing.T) {
		RegisterTestingT(t)
		for _, key := range []string{"a[b", "a]b", "[a]", "a[][b]", "a[b]c"} {
			_, err := apiFieldsJSON([]apiField{{key, "v"}})
			Expect(err).To(MatchError(ContainSubstring("invalid field key")), key)
		}
	})
}

func TestAPIFieldValues(t *testing.T) {
	RegisterTestingT(t)
	got := apiFieldValues([]apiField{
		{"names", "Ivan Petrov"}, {"page", int64(2)}, {"only_ids", false}, {"x", nil}, {"ids[]", "1"}, {"ids[]", "2"},
	})

	// Bracketed keys stay literal: Rails nests them on its side.
	Expect(got.Encode()).To(Equal("ids%5B%5D=1&ids%5B%5D=2&names=Ivan+Petrov&only_ids=false&page=2&x="))
}

func TestResolveAPIURL(t *testing.T) {
	cases := []struct {
		name     string
		endpoint string
		sandbox  bool
		want     string
	}{
		{"bare id", "profile-123", false, "https://www.geni.com/api/profile-123"},
		{"leading slash", "/profile-123/immediate-family", false, "https://www.geni.com/api/profile-123/immediate-family"},
		{"api prefix", "api/profile-123", false, "https://www.geni.com/api/profile-123"},
		{"slash api prefix", "/api/profile/search?names=Ivan", false, "https://www.geni.com/api/profile/search?names=Ivan"},
		{"sandbox bare", "user", true, "https://sandbox.geni.com/api/user"},
		{"prod URL", "https://www.geni.com/api/profile-1?fields=id", false, "https://www.geni.com/api/profile-1?fields=id"},
		{"sandbox URL", "https://sandbox.geni.com/api/profile-1", true, "https://sandbox.geni.com/api/profile-1"},
		{"sandbox API host", "https://api.sandbox.geni.com/profile/search?names=x&page=2", true, "https://sandbox.geni.com/api/profile/search?names=x&page=2"},
		{"strips a token", "https://www.geni.com/api/profile/search?access_token=secret&page=2", false, "https://www.geni.com/api/profile/search?page=2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)
			got, err := resolveAPIURL(tc.endpoint, tc.sandbox)

			Expect(err).ToNot(HaveOccurred())
			Expect(got.String()).To(Equal(tc.want))
		})
	}

	t.Run("refuses a foreign host so the token never leaves geni.com", func(t *testing.T) {
		RegisterTestingT(t)
		_, err := resolveAPIURL("https://example.com/api/profile-1", false)
		Expect(err).To(MatchError(ContainSubstring("not a production API URL")))
	})

	t.Run("refuses a production URL under -sandbox, and says why", func(t *testing.T) {
		RegisterTestingT(t)
		_, err := resolveAPIURL("https://www.geni.com/api/profile-1", true)
		Expect(err).To(MatchError(ContainSubstring("drop -sandbox")))

		_, err = resolveAPIURL("https://sandbox.geni.com/api/profile-1", false)
		Expect(err).To(MatchError(ContainSubstring("add -sandbox")))
	})

	t.Run("refuses an empty endpoint", func(t *testing.T) {
		RegisterTestingT(t)
		_, err := resolveAPIURL("/api/", false)
		Expect(err).To(HaveOccurred())
	})
}

func TestResolveWebURL(t *testing.T) {
	cases := []struct {
		name string
		path string
		want string
	}{
		{"absolute path", "/list/data_conflicts", "https://www.geni.com/list/data_conflicts"},
		{"relative path with query", "list/data_conflicts?page=2", "https://www.geni.com/list/data_conflicts?page=2"},
		{"full URL", "https://www.geni.com/merge/resolve/abc", "https://www.geni.com/merge/resolve/abc"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)
			got, err := resolveWebURL(tc.path, "https://www.geni.com")

			Expect(err).ToNot(HaveOccurred())
			Expect(got.String()).To(Equal(tc.want))
		})
	}

	t.Run("refuses another host so the cookies never leave geni.com", func(t *testing.T) {
		RegisterTestingT(t)
		_, err := resolveWebURL("https://evil.example/list", "https://www.geni.com")
		Expect(err).To(MatchError(ContainSubstring("https://www.geni.com")))

		_, err = resolveWebURL("https://www.geni.com.evil.example/list", "https://www.geni.com")
		Expect(err).To(HaveOccurred())
	})
}
