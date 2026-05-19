package profile

import (
	"net/http"
	"strings"
)

// FieldsQueryValue is the comma-separated list passed as the `fields=`
// query param on profile-returning endpoints. It enumerates exactly
// the Profile fields the client knows how to decode, so the API
// doesn't waste bandwidth on fields the client would ignore.
const FieldsQueryValue = "id,guid,first_name,last_name,middle_name,maiden_name,display_name,nicknames,names,gender,title,suffix,occupation,birth,baptism,death,burial,cause_of_death,current_residence,about_me,detail_strings,unions,project_ids,is_alive,public,deleted,merged_into,updated_at,created_at"

// AddFields sets the `fields=` query param on req to the canonical
// Profile fields list. Sub-packages that build profile-returning
// requests call this so the wire shape stays consistent.
func AddFields(req *http.Request) {
	query := req.URL.Query()
	query.Add("fields", FieldsQueryValue)
	req.URL.RawQuery = query.Encode()
}

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
