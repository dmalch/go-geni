package profile

import "strings"

// StripURLs removes the API URL prefix from each entry in p.Unions
// (and in the future, any other URL-shaped fields the server returns
// despite only_ids=true). Callers pass the API host returned by
// transport.Client.APIURL(); profile/ does not depend on transport.
//
// only_ids=true does not honour the Unions field, so listings of
// Profile that surface through bulk endpoints need this post-process
// step to coerce raw URLs back to bare ids.
func StripURLs(p *Profile, apiURL string) {
	for i, union := range p.Unions {
		p.Unions[i] = strings.Replace(union, apiURL, "", 1)
	}
}
