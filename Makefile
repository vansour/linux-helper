.PHONY: build build-all clean install release

BINARY=linux-helper
DIST=dist
VERSION=$(shell cat VERSION 2>/dev/null || echo "dev")
LDFLAGS=-ldflags="-s -w"

# Default build (current platform)
build:
	go build $(LDFLAGS) -o $(DIST)/$(BINARY) .

# Cross-compile for common Linux architectures
build-all:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(DIST)/$(BINARY)-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(DIST)/$(BINARY)-linux-arm64 .
	GOOS=linux GOARCH=386 go build $(LDFLAGS) -o $(DIST)/$(BINARY)-linux-386 .
	@echo "Built binaries:"
	@ls -lh $(DIST)/

# Compress with upx (if available)
compress: build-all
	@if command -v upx &>/dev/null; then \
		upx --best $(DIST)/$(BINARY)-linux-* 2>/dev/null; \
		ls -lh $(DIST)/; \
	fi

# Clean build artifacts
clean:
	rm -rf $(DIST)/

# Test
test:
	go test ./...

# Format all Go code
fmt:
	go fmt ./...

# Vet all packages
vet:
	go vet ./...

# Run lint
lint:
	@if command -v golangci-lint &>/dev/null; then \
		golangci-lint run ./...; \
	else \
		go vet ./...; \
	fi
