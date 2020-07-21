PKG     = github.com/sapcc/absent-metrics-operator
PREFIX := /usr

VERSION         := $(shell git describe --long --abbrev=6)
GIT_COMMIT_HASH := $(shell git rev-parse --verify HEAD)

# NOTE: This repo uses Go modules, and uses a synthetic GOPATH at
# $(CURDIR)/.gopath that is only used for the build cache. $GOPATH/src/ is
# empty.
GO            := GOPATH=$(CURDIR)/.gopath GOBIN=$(CURDIR)/build go
GO_BUILDFLAGS :=
GO_LDFLAGS    := -s -w -X $(PKG)/internal/version.Version=$(VERSION) -X $(PKG)/internal/version.GitCommitHash=$(GIT_COMMIT_HASH)

all: build

build: FORCE
	$(GO) install $(GO_BUILDFLAGS) -ldflags '$(GO_LDFLAGS)' '$(PKG)'

install: FORCE build
	install -D -m 0755 build/absent-metrics-operator "$(DESTDIR)$(PREFIX)/bin/absent-metrics-operator"

static-check: FORCE
	@printf "\e[1;34m>> golangci-lint\e[0m\n"
	@command -v golangci-lint >/dev/null 2>&1 || { echo >&2 "Error: golangci-lint is not installed. See: https://golangci-lint.run/usage/install/"; exit 1; }
	golangci-lint run

check: FORCE static-check
	@printf "\e[1;32m>> All tests successful\e[0m\n"

clean: FORCE
	rm -rf -- build/*

vendor: FORCE
	$(GO) mod tidy -v
	$(GO) mod vendor
	$(GO) mod verify

.PHONY: FORCE
