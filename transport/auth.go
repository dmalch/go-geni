package transport

import (
	"fmt"
	"net/http"
	"net/url"
)

// addStandardHeadersAndQueryParams injects the access_token,
// api_version, and only_ids query params into req, and sets the
// Accept / User-Agent / Content-Type headers. Content-Type is only
// set when the caller hasn't already done so — multipart upload
// endpoints (e.g. photo/add) pre-set their own header with the
// boundary parameter and would otherwise end up with two conflicting
// Content-Type values.
func (c *Client) addStandardHeadersAndQueryParams(req *http.Request) error {
	query := req.URL.Query()

	token, err := c.tokenSource.Token()
	if err != nil {
		return fmt.Errorf("error getting token: %w", err)
	}

	query.Add("access_token", token.AccessToken)
	query.Add("api_version", APIVersion)
	// The returned data structures will contain urls to other objects by
	// default, unless the request includes 'only_ids=true.' Passing
	// only_ids will force the system to return ids only.
	query.Add("only_ids", "true")

	req.URL.RawQuery = query.Encode()
	req.Header.Add("Accept", "application/json")
	if req.Header.Get("Content-Type") == "" {
		req.Header.Add("Content-Type", "application/json")
	}
	req.Header.Add("User-Agent", "terraform-provider-genealogy/0.1")

	return nil
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
