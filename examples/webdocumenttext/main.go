// Command webdocumenttext is a smoke example for the AJAX Web
// client's document-text endpoints. With one argument it prints the
// current text body of a document; with two, it overwrites it.
//
// Run:
//
//	GENI_WEB_COOKIES="_geni_session=..." go run ./examples/webdocumenttext <doc-guid>
//	GENI_WEB_COOKIES="_geni_session=..." go run ./examples/webdocumenttext <doc-guid> "new body"
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/dmalch/go-geni/web"
	"github.com/dmalch/go-geni/web/document"
)

func main() {
	if len(os.Args) < 2 || len(os.Args) > 3 {
		log.Fatalf("usage: %s <doc-guid> [new-text]", os.Args[0])
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
	d := document.NewClient(c)

	if len(os.Args) == 2 {
		text, err := d.GetText(context.Background(), guid)
		if err != nil {
			log.Fatalf("GetText(%q): %v", guid, err)
		}
		fmt.Println(text)
		return
	}

	if err := d.SaveText(context.Background(), guid, os.Args[2]); err != nil {
		log.Fatalf("SaveText(%q): %v", guid, err)
	}
	fmt.Println("ok")
}
