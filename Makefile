.PHONY: all build test run clean coverage lint help

# ─── Variables ────────────────────────────────────────────────────────────────

BINARY    := t-browser
GO        := go
GOFLAGS   := -ldflags="-s -w"
COVEROUT  := coverage.out

# Detect OS for the binary name
ifeq ($(OS),Windows_NT)
	BINARY := t-browser.exe
endif

# ─── Default ──────────────────────────────────────────────────────────────────

all: clean build test

# ─── Build ────────────────────────────────────────────────────────────────────

build:
	$(GO) build $(GOFLAGS) -o $(BINARY) .

build-race:
	$(GO) build -race -o $(BINARY)-race .

# ─── Run ──────────────────────────────────────────────────────────────────────

run: build
	./$(BINARY) $(URL)

# ─── Test ────────────────────────────────────────────────────────────────────

test:
	$(GO) test ./... -v -count=1

test-short:
	$(GO) test ./... -short -count=1

test-race:
	$(GO) test ./... -race -count=1

# ─── Coverage ────────────────────────────────────────────────────────────────

coverage:
	$(GO) test ./... -coverprofile=$(COVEROUT) -count=1
	$(GO) tool cover -func=$(COVEROUT)

coverage-html:
	$(GO) test ./... -coverprofile=$(COVEROUT) -count=1
	$(GO) tool cover -html=$(COVEROUT)

coverage-clean:
	rm -f $(COVEROUT)

# ─── Lint ─────────────────────────────────────────────────────────────────────

lint:
	$(GO) vet ./...

# ─── Clean ────────────────────────────────────────────────────────────────────

clean:
	rm -f $(BINARY) $(BINARY)-race $(COVEROUT)

# ─── Dependencies ─────────────────────────────────────────────────────────────

tidy:
	$(GO) mod tidy

vendor:
	$(GO) mod vendor

deps-update:
	$(GO) get -u ./...
	$(GO) mod tidy

# ─── Help ─────────────────────────────────────────────────────────────────────

help:
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@echo "  all              Run clean → test (default)"
	@echo "  build            Compile the binary"
	@echo "  run URL=https://… Build and launch the browser"
	@echo "  test             Run all tests with verbose output"
	@echo "  test-short       Run tests excluding slow (timed) tests"
	@echo "  test-race        Run tests with the race detector enabled"
	@echo "  coverage         Run tests and print per-function coverage"
	@echo "  coverage-html    Open an interactive HTML coverage report"
	@echo "  lint             Run go vet"
	@echo "  tidy             Run go mod tidy"
	@echo "  clean            Remove build artefacts"
	@echo "  help             Show this message"
