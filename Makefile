GO      ?= go
BINARY  := repoman
PKG     := ./cmd/repoman
VERSION := $(shell cat VERSION 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=v$(VERSION)

# Matches .goreleaser.yaml's build matrix exactly -- keep the two in sync
# if either changes. DragonFly BSD has no arm64 -- Go itself has no
# dragonfly/arm64 port (confirmed via `go tool dist list`), so it's
# listed once here, amd64 only, rather than as part of a uniform
# cartesian product that would try (and fail) to build it.
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64 \
             freebsd/amd64 freebsd/arm64 openbsd/amd64 openbsd/arm64 netbsd/amd64 netbsd/arm64 \
             dragonfly/amd64

DIST := dist

.PHONY: all
all: build

.PHONY: build
build: ## Build the repoman binary for the current platform into ./repoman
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BINARY) $(PKG)

.PHONY: run
run: build ## Build, then print the version (fast sanity check the binary works)
	./$(BINARY) version

.PHONY: selftest
selftest: build ## Build, then run the 75-check acceptance gate. Red gate means don't trust this build.
	./$(BINARY) selftest

.PHONY: vet
vet: ## go vet ./...
	$(GO) vet ./...

.PHONY: fmt
fmt: ## Apply gofmt across the tree
	gofmt -w .

.PHONY: fmt-check
fmt-check: ## Fail if any file isn't gofmt-clean, without modifying anything
	@out="$$(gofmt -l .)"; \
	if [ -n "$$out" ]; then \
		echo "gofmt would reformat:"; \
		echo "$$out"; \
		exit 1; \
	fi

.PHONY: verify
verify: build vet fmt-check selftest ## The full local gate: build, vet, gofmt-check, selftest. Same checks CI runs on every push.

.PHONY: cross
cross: ## Cross-compile every release target into dist/ (CGO_ENABLED=0, matches .goreleaser.yaml)
	@mkdir -p $(DIST)
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; \
		ext=""; \
		if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
		out=$(DIST)/repoman-$$os-$$arch$$ext; \
		echo "  $$os/$$arch -> $$out"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch $(GO) build -ldflags "$(LDFLAGS)" -o $$out $(PKG) || exit 1; \
	done
	@cd $(DIST) && sha256sum repoman-* > checksums.txt
	@echo "done -- $(DIST)/ has one binary per platform plus checksums.txt"

.PHONY: release-dry-run
release-dry-run: ## Local GoReleaser dry run: same build as a real release, nothing published. Requires goreleaser on PATH.
	@command -v goreleaser >/dev/null 2>&1 || { \
		echo "goreleaser not on PATH -- install it yourself (this target" ; \
		echo "deliberately doesn't auto-install it: it pulls a large" ; \
		echo "dependency tree, and that's not a decision to make silently" ; \
		echo "from inside a Makefile). See https://goreleaser.com/install/" ; \
		exit 1; \
	}
	goreleaser release --snapshot --skip=publish --clean

.PHONY: clean
clean: ## Remove build artifacts (the binary, dist/) -- never touches VERSION/CHANGELOG.md or anything tracked
	rm -f $(BINARY)
	rm -rf $(DIST)

.PHONY: help
help: ## List all targets with their one-line descriptions
	@grep -E '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN{FS=":.*## "} {printf "  %-16s %s\n", $$1, $$2}'
