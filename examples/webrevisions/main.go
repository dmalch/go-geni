// Command webrevisions is a smoke example for the AJAX Web client's
// revision endpoint. It lists the revision IDs of a profile by GUID
// using a session cookie taken from a logged-in browser.
//
// Run:
//
//	GENI_WEB_COOKIES="_geni_session=...; remember_user_token=..." \
//	    go run ./examples/webrevisions <profile-guid>
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/dmalch/go-geni/web"
	"github.com/dmalch/go-geni/web/revision"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("usage: %s <profile-guid>", os.Args[0])
	}
	guid := os.Args[1]

	cookies := os.Getenv("GENI_WEB_COOKIES")
	if cookies == "" {
		log.Fatal("set GENI_WEB_COOKIES to the Cookie header value from a logged-in geni.com browser tab")
	}

	c, err := web.NewClient(web.Options{Cookies: web.CookiesFromHeader(cookies)})
	if err != nil {
		log.Fatal(err)
	}

	ids, err := revision.NewClient(c).ForProfile(context.Background(), guid)
	if err != nil {
		log.Fatalf("ForProfile(%q): %v", guid, err)
	}

	out, _ := json.MarshalIndent(ids, "", "  ")
	fmt.Println(string(out))
}
