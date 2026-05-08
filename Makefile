.PHONY: build install uninstall check test hooks release clean help

BIN_NAME := tmux-login
BIN := bin/$(BIN_NAME)
PKG := ./cmd/tmux-login
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X github.com/yigitkonur/tmux-login/internal/version.Version=$(VERSION) -X github.com/yigitkonur/tmux-login/internal/version.Commit=$(COMMIT) -X github.com/yigitkonur/tmux-login/internal/version.Date=$(DATE)
GOFLAGS := -trimpath -ldflags "$(LDFLAGS)"

help:
	@echo "targets:"
	@echo "  build      build the tmux-login binary at $(BIN)"
	@echo "  install    run install.sh (uses bin/$(BIN_NAME) if present)"
	@echo "  uninstall  run uninstall.sh"
	@echo "  check      gofmt + go vet + go test + shellcheck + roundtrip + runtime + perf"
	@echo "  test       alias for check"
	@echo "  hooks      point git at .githooks/ (enables pre-push make check)"
	@echo "  release    build for darwin/linux x amd64/arm64 into dist/"
	@echo "  clean      remove bin/ and dist/"

build:
	@mkdir -p bin
	go build $(GOFLAGS) -o $(BIN) $(PKG)
	@echo "built $(BIN) ($(VERSION))"

install: build
	sh ./install.sh --no-install-deps --bin-from="$$(pwd)/$(BIN)"

uninstall:
	sh ./uninstall.sh

hooks:
	git config core.hooksPath .githooks
	@echo "hooks: .git now runs .githooks/* — pre-push gates on make check"

check test: build
	@gofmt -l . | (! grep .) && echo "gofmt: OK" || (echo "gofmt: FAILED — run gofmt -w ."; exit 1)
	@go vet ./... && echo "go vet: OK"
	@go test ./... && echo "go test: OK"
	@sh -n install.sh                && echo "sh -n:  install.sh                OK"
	@sh -n uninstall.sh              && echo "sh -n:  uninstall.sh              OK"
	@sh -n test/roundtrip.sh         && echo "sh -n:  test/roundtrip.sh         OK"
	@sh -n test/runtime.sh           && echo "sh -n:  test/runtime.sh           OK"
	@sh -n test/perf.sh              && echo "sh -n:  test/perf.sh              OK"
	@sh -n test/fixtures/tmux-stub.sh && echo "sh -n:  tmux-stub.sh              OK"
	@sh -n test/fixtures/fzf-stub.sh  && echo "sh -n:  fzf-stub.sh               OK"
	@zsh -n share/login-hook.zsh     && echo "zsh -n: share/login-hook.zsh      OK"
	@if command -v shellcheck >/dev/null 2>&1; then \
		shellcheck --shell=sh install.sh uninstall.sh test/roundtrip.sh test/runtime.sh test/perf.sh test/fixtures/tmux-stub.sh test/fixtures/fzf-stub.sh \
		  && echo "shellcheck: OK"; \
	else \
		echo "shellcheck: not installed (skipped)"; \
	fi
	sh test/roundtrip.sh
	sh test/runtime.sh
	@if [ "$$SKIP_PERF" != "1" ]; then sh test/perf.sh; else echo "perf: skipped (SKIP_PERF=1)"; fi

release:
	@mkdir -p dist
	@for os in darwin linux; do for arch in amd64 arm64; do \
		echo "building $$os/$$arch"; \
		GOOS=$$os GOARCH=$$arch go build $(GOFLAGS) -o dist/$(BIN_NAME)-$$os-$$arch $(PKG); \
	done; done
	@echo "release artifacts in dist/"

clean:
	rm -rf bin/ dist/
