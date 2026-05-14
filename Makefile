.PHONY: build vet test test-acceptance lint check

build:
	go build ./...

vet:
	go vet ./...

# Unit + Ginkgo integration suite (everything but test/acceptance, which
# needs a real OAuth token and a network call to Geni's sandbox).
test:
	go test ./...

# Hits sandbox.geni.com. Set GENI_OAUTH=1 to authorize via a browser
# (and cache the token under ~/.genealogy/geni_sandbox_token.json for
# subsequent runs). Override with GENI_ACCESS_TOKEN=<token> for
# non-interactive runs. Self-skips when neither path is configured.
test-acceptance:
	GENI_OAUTH=1 go test -v -count=1 ./test/acceptance/...

lint:
	golangci-lint run ./...

# `make check` runs the same gates CI runs. Equivalent to a manual pre-push.
check: build vet lint test
