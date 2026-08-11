BINARY  := gsw
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test vet fmt install clean run

build:
	go build -ldflags '$(LDFLAGS)' -o bin/$(BINARY) ./cmd/gsw

test:
	go test ./...

vet:
	go vet ./...

fmt:
	go fmt ./...

# Installs to ~/.local/bin, which needs no sudo.
install: build
	install -Dm755 bin/$(BINARY) $(HOME)/.local/bin/$(BINARY)
	@echo "Installed to $(HOME)/.local/bin/$(BINARY)"
	@command -v $(BINARY) >/dev/null 2>&1 || echo "warning: $(HOME)/.local/bin is not on your PATH"

# No frontend build step: web/app is hand-written and embedded via go:embed, so
# `build` above already includes the UI and nothing here needs node.

run: build
	./bin/$(BINARY)

clean:
	rm -rf bin
