PKG     = github.com/sapcc/absent-metrics-operator
PREFIX := /usr

GOOS   = $(shell go env GOOS)
GOARCH = $(shell go env GOARCH)
KUBEBUILDER_RELEASE_VERSION = $(shell cat test/.kubebuilder-version)

VERSION     := $(shell git describe --abbrev=7)
COMMIT_HASH := $(shell git rev-parse --verify HEAD)
BUILD_DATE  := $(shell date -u +"%Y-%m-%dT%H:%M:%S%Z")

GO            := GOBIN=$(CURDIR)/build go
GO_BUILDFLAGS :=
GO_LDFLAGS    := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT_HASH) -X main.date=$(BUILD_DATE)

all: FORCE build/absent-metrics-operator

build/absent-metrics-operator: FORCE | build
	$(GO) install $(GO_BUILDFLAGS) -ldflags '$(GO_LDFLAGS)' '$(PKG)'

install: FORCE build/absent-metrics-operator
	install -D -m 0755 build/absent-metrics-operator "$(DESTDIR)$(PREFIX)/bin/absent-metrics-operator"

# Run all checks
check: FORCE build/absent-metrics-operator lint build/cover.html
	@printf "\e[1;32m>> All checks successful\e[0m\n"

lint: FORCE
	@printf "\e[1;34m>> golangci-lint\e[0m\n"
	@command -v golangci-lint >/dev/null 2>&1 || { echo >&2 "Error: golangci-lint is not installed. See: https://golangci-lint.run/usage/install/"; exit 1; }
	golangci-lint run

# Run unit tests
test: FORCE | test/bin
	@printf "\e[1;34m>> go test\e[0m\n"
	$(GO) test $(GO_BUILDFLAGS) -ldflags '$(GO_LDFLAGS)' $(PKG)/test

# Test with coverage
test-coverage: FORCE build/cover.out
build/cover.out: FORCE | build test/bin
	@printf "\e[1;34m>> go test with coverage\e[0m\n"
	$(GO) test $(GO_BUILDFLAGS) -ldflags '$(GO_LDFLAGS)' -failfast -race -covermode=atomic -coverpkg=$(PKG)/internal/controller -coverprofile=$@  $(PKG)/test
build/cover.html: build/cover.out
	$(GO) tool cover -html $< -o $@

build/release-info: CHANGELOG.md | build
	$(GO) run $(GO_BUILDFLAGS) tools/releaseinfo.go $< $(shell git describe --abbrev=0) > $@

build:
	mkdir $@

# Download the kubebuilder control plane binaries
test/bin:
	mkdir $@
	# Download kubebuilder and extract it to tmp
	curl -L https://go.kubebuilder.io/dl/$(KUBEBUILDER_RELEASE_VERSION)/$(GOOS)/$(GOARCH) | tar -xz -C /tmp/
	# Move to test/bin
	mv /tmp/kubebuilder*/bin/* test/bin/

clean: FORCE
	rm -rf -- build test/bin

vendor: FORCE
	$(GO) mod tidy -v
	$(GO) mod vendor
	$(GO) mod verify

.PHONY: FORCE
