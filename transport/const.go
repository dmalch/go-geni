package transport

const APIVersion = "1"

const (
	geniProdURL       = "https://www.geni.com/"
	geniSandboxURL    = "https://sandbox.geni.com/"
	geniProdAPIURL    = "https://www.geni.com/api/"
	geniSandboxAPIURL = "https://api.sandbox.geni.com/"
)

// BaseURL returns the prod or sandbox HTTP host (with trailing slash).
// Callers append "api/<path>" to build full request URLs.
func BaseURL(useSandboxEnv bool) string {
	if useSandboxEnv {
		return geniSandboxURL
	}
	return geniProdURL
}

// APIURL returns the prod or sandbox API host (with "api/" suffix and
// trailing slash). Used when stripping URL prefixes from response
// bodies that ignored only_ids=true — e.g. ProfileResponse.Unions.
func APIURL(useSandboxEnv bool) string {
	if useSandboxEnv {
		return geniSandboxAPIURL
	}
	return geniProdAPIURL
}
