// Command geni is a command-line client for the Geni.com genealogy API.
//
// It is a thin façade over the github.com/dmalch/go-geni library:
// "geni login" runs a browser-based OAuth handshake and caches the
// token, and the read commands ("geni profile get", "geni whoami", …)
// print JSON results to stdout.
//
// Run "geni help" for the full command list.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

// globalOpts holds the flags and I/O streams shared by every command.
type globalOpts struct {
	sandbox bool
	browser string // -browser / GENI_WEB_BROWSER — narrows cookie reads to one backend
	stdin   io.Reader
	stdout  io.Writer
	stderr  io.Writer
}

// command is a node in the command tree: a leaf when run is set, a
// group of subcommands when sub is set.
type command struct {
	summary string
	run     func(ctx context.Context, g *globalOpts, args []string) error
	sub     map[string]*command
}

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run parses args, dispatches to a command, and returns a process
// exit code: 0 success, 1 command error, 2 usage error.
func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	g := &globalOpts{stdin: stdin, stdout: stdout, stderr: stderr}

	fs := flag.NewFlagSet("geni", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.BoolVar(&g.sandbox, "sandbox", false, "use sandbox.geni.com instead of production")
	fs.StringVar(&g.browser, "browser", "", "AJAX cookie source (chrome|edge|brave|arc|chromium|vivaldi|opera|firefox|safari); empty = try all")
	fs.Usage = func() { printUsage(stderr) }
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if os.Getenv("GENI_USE_SANDBOX") == "true" {
		g.sandbox = true
	}
	if g.browser == "" {
		g.browser = os.Getenv("GENI_WEB_BROWSER")
	}

	rest := fs.Args()
	if len(rest) == 0 {
		printUsage(stderr)
		return 2
	}

	cmd, ok := commandTree()[rest[0]]
	if !ok {
		_, _ = fmt.Fprintf(stderr, "geni: unknown command %q\n\n", rest[0])
		printUsage(stderr)
		return 2
	}
	path := rest[0]
	rest = rest[1:]

	for cmd.sub != nil {
		if len(rest) == 0 {
			_, _ = fmt.Fprintf(stderr, "geni %s: expected a subcommand\n\n", path)
			printUsage(stderr)
			return 2
		}
		next, ok := cmd.sub[rest[0]]
		if !ok {
			_, _ = fmt.Fprintf(stderr, "geni %s: unknown subcommand %q\n\n", path, rest[0])
			printUsage(stderr)
			return 2
		}
		cmd = next
		path += " " + rest[0]
		rest = rest[1:]
	}

	if cmd.run == nil {
		printUsage(stderr)
		return 2
	}
	if err := cmd.run(ctx, g, rest); err != nil {
		_, _ = fmt.Fprintf(stderr, "geni %s: %v\n", path, err)
		return 1
	}
	return 0
}
