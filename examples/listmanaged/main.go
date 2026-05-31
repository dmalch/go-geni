// Command listmanaged streams every Geni profile managed by the
// authenticated user as one JSON object per line on stdout. Progress
// is logged to stderr.
//
// Auth (matches terraform-provider-genealogy):
//   - If GENI_ACCESS_TOKEN is set, use it directly.
//   - Otherwise, OAuth implicit flow via browser; token cached at
//     ~/.genealogy/geni_token.json (prod) or geni_sandbox_token.json (sandbox).
//
// Run (production):
//
//	go run ./examples/listmanaged > managed.jsonl
//
// Run (sandbox):
//
//	go run ./examples/listmanaged -sandbox > managed.jsonl
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"time"

	geni "github.com/dmalch/go-geni"
	"github.com/dmalch/go-geni/auth"
	"golang.org/x/oauth2"
)

func main() {
	sandbox := flag.Bool("sandbox", false, "use sandbox.geni.com instead of production")
	maxPages := flag.Int("max-pages", 0, "stop after this many pages (0 = no limit, for debugging)")
	flag.Parse()

	if os.Getenv("GENI_USE_SANDBOX") == "true" {
		*sandbox = true
	}

	tokenSource, err := newTokenSource(*sandbox)
	if err != nil {
		log.Fatalf("token source: %v", err)
	}

	client := geni.NewClient(tokenSource, *sandbox)
	ctx := context.Background()
	enc := json.NewEncoder(os.Stdout)

	start := time.Now()
	page := 1
	total := 0
	declared := -1
	for {
		resp, err := client.User().ManagedProfiles(ctx, page)
		if err != nil {
			log.Fatalf("GetManagedProfiles(page=%d): %v", page, err)
		}
		if declared < 0 && resp.TotalCount > 0 {
			declared = resp.TotalCount
		}
		for i := range resp.Results {
			if err := enc.Encode(&resp.Results[i]); err != nil {
				log.Fatalf("encode profile %s: %v", resp.Results[i].ID, err)
			}
			total++
		}
		fmt.Fprintf(os.Stderr, "page %d: %d profiles (total so far %d / %d)\n",
			resp.Page, len(resp.Results), total, declared)
		if resp.NextPage == "" || len(resp.Results) == 0 {
			break
		}
		if *maxPages > 0 && page >= *maxPages {
			fmt.Fprintf(os.Stderr, "stopping at -max-pages=%d\n", *maxPages)
			break
		}
		page++
	}
	fmt.Fprintf(os.Stderr, "done: %d profiles in %s\n", total, time.Since(start).Round(time.Second))
}

// newTokenSource mirrors the terraform-provider-genealogy chain so the
// same cached token at ~/.genealogy/geni_token.json is reused.
func newTokenSource(useSandbox bool) (oauth2.TokenSource, error) {
	if t := os.Getenv("GENI_ACCESS_TOKEN"); t != "" {
		return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: t}), nil
	}

	cachePath, err := tokenCacheFilePath(useSandbox)
	if err != nil {
		return nil, err
	}

	cfg := &oauth2.Config{
		ClientID: clientID(useSandbox),
		Endpoint: oauth2.Endpoint{
			AuthURL: geni.BaseURL(useSandbox) + "platform/oauth/authorize",
		},
	}
	return oauth2.ReuseTokenSource(nil,
		auth.NewCachingTokenSource(cachePath, auth.NewAuthTokenSource(cfg))), nil
}

func tokenCacheFilePath(useSandbox bool) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home dir: %w", err)
	}
	name := "geni_token.json"
	if useSandbox {
		name = "geni_sandbox_token.json"
	}
	return path.Join(home, ".genealogy", name), nil
}

func clientID(useSandbox bool) string {
	if useSandbox {
		return "8"
	}
	return "1855"
}
