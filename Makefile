PKG     = github.com/sapcc/absent-metrics-operator
PREFIX := /usr

# TODO: uncomment when at least one tag exists
# VERSION         := $(shell git describe --long --abbrev=7)
GIT_COMMIT_HASH := $(shell git rev-parse --verify HEAD)

GO            := GOBIN=$(CURDIR)/build go
GO_BUILDFLAGS :=
GO_LDFLAGS    := -s -w -X main.version=$(VERSION) -X main.gitCommitHash=$(GIT_COMMIT_HASH)

# Which packages to test with `go test`?
GO_TESTPKGS   := $(shell $(GO) list $(GO_BUILDFLAGS) -f '{{if .TestGoFiles}}{{.ImportPath}}{{end}}' ./...)
# Which packages to measure test coverage for?
GO_COVERPKGS  := $(shell $(GO) list $(GO_BUILDFLAGS) $(PKG) $(PKG)/internal/... )
# Output files from `go test`.
GO_COVERFILES := $(patsubst %,build/%.cover.out,$(subst /,_,$(GO_TESTPKGS)))

# This is needed for substituting spaces with commas.
space := $(null) $(null)
comma := ,

all: FORCE build/absent-metrics-operator

build/absent-metrics-operator: FORCE | build
	$(GO) install $(GO_BUILDFLAGS) -ldflags '$(GO_LDFLAGS)' '$(PKG)'

install: FORCE build/absent-metrics-operator
	install -D -m 0755 build/absent-metrics-operator "$(DESTDIR)$(PREFIX)/bin/absent-metrics-operator"

lint: FORCE
	@printf "\e[1;34m>> golangci-lint\e[0m\n"
	@command -v golangci-lint >/dev/null 2>&1 || { echo >&2 "Error: golangci-lint is not installed. See: https://golangci-lint.run/usage/install/"; exit 1; }
	golangci-lint run

# Run all checks
check: FORCE build/absent-metrics-operator lint build/cover.html
	@printf "\e[1;32m>> All checks successful\e[0m\n"

# Run unit tests
test: FORCE
	@printf "\e[1;34m>> go test\e[0m\n"
	$(GO) test $(GO_BUILDFLAGS) -ldflags '$(GO_LDFLAGS)' $(GO_TESTPKGS)

# Test with coverage
test-coverage: FORCE build/cover.out
build/%.cover.out: FORCE | build
	@printf "\e[1;34m>> go test $(subst _,/,$*)\e[0m\n"
	$(GO) test $(GO_BUILDFLAGS) -ldflags '$(GO_LDFLAGS)' -failfast -race -coverprofile=$@ -covermode=atomic -coverpkg=$(subst $(space),$(comma),$(GO_COVERPKGS)) $(subst _,/,$*)
build/cover.out: $(GO_COVERFILES)
	$(GO) run $(GO_BUILDFLAGS) tools/gocovcat/main.go $(GO_COVERFILES) > $@
build/cover.html: build/cover.out
	$(GO) tool cover -html $< -o $@

build:
	mkdir $@

clean: FORCE
	rm -rf -- build/*

vendor: FORCE
	$(GO) mod tidy -v
	$(GO) mod vendor
	$(GO) mod verify

.PHONY: FORCE
