# Local equivalents of the checks .github/workflows/ci.yml runs. Keeping them here
# means you can reproduce a CI failure without pushing.

# Must match the `version:` input of golangci/golangci-lint-action in
# .github/workflows/ci.yml. Dependabot bumps the action but not this file, so the two
# can drift silently — update both together.
GOLANGCI_LINT_VERSION ?= v2.12.2

GOBIN ?= $(shell go env GOPATH)/bin

.PHONY: help verify build vet fmt fmt-check test test-integration lint lint-fix tools

help: ## List the available targets
	@grep -hE '^[a-z-]+:.*?## ' $(MAKEFILE_LIST) | awk -F':.*?## ' '{printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

verify: build vet fmt-check ## Everything the CI build job checks

build: ## Compile every package
	go build ./...

vet: ## go vet, with and without the integration build tag
	go vet ./...
	# The integration-tagged files are excluded from every other build, so without
	# this they could stop compiling unnoticed.
	go vet -tags=integration ./...

fmt: ## Rewrite any badly formatted file in place
	gofmt -w .

fmt-check: ## Fail if anything is unformatted
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt reported unformatted files; run 'make fmt':"; \
		echo "$$unformatted"; \
		exit 1; \
	fi; \
	echo "gofmt: all files formatted"

test: ## Unit tests, race detector on, cache bypassed
	go test ./... -race -count=1

test-integration: ## Database-backed tests; needs a running Docker daemon
	# On a non-default Docker socket (colima, Rancher Desktop, rootless) export
	# DOCKER_HOST and TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE first — see the README.
	go test -tags=integration ./... -race -count=1

lint: ## Run golangci-lint exactly as CI does
	golangci-lint run

lint-fix: ## Run golangci-lint and apply the fixes it can make itself
	golangci-lint run --fix

tools: ## Install golangci-lint at the pinned version into GOPATH/bin
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh \
		| sh -s -- -b "$(GOBIN)" $(GOLANGCI_LINT_VERSION)
