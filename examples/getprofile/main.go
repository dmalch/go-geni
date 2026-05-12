// Command getprofile is a minimal smoke example that constructs a go-geni
// Client against the sandbox and fetches a single profile.
//
// Run:
//
//	GENI_ACCESS_TOKEN=... go run ./examples/getprofile profile-1
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/dmalch/go-geni"
	"golang.org/x/oauth2"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("usage: %s <profile-id-or-guid>", os.Args[0])
	}
	profileID := os.Args[1]

	token := os.Getenv("GENI_ACCESS_TOKEN")
	if token == "" {
		log.Fatal("set GENI_ACCESS_TOKEN (sandbox token; see https://sandbox.geni.com/platform/developer/api_explorer)")
	}

	client := geni.NewClient(
		oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token}),
		true, // sandbox
	)

	profile, err := client.GetProfile(context.Background(), profileID)
	if err != nil {
		log.Fatalf("GetProfile(%q): %v", profileID, err)
	}

	fmt.Printf("id:    %s\n", profile.Id)
	fmt.Printf("guid:  %s\n", profile.Guid)
	fmt.Printf("name:  %s %s\n", derefString(profile.FirstName), derefString(profile.LastName))
	fmt.Printf("alive: %t\n", profile.IsAlive)
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
