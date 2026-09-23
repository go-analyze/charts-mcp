export GO111MODULE = on

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X github.com/go-analyze/charts-mcp/pkg/config.Version=$(VERSION)"

.PHONY: default build test test-cover bench fmt-changed lint clean

default: build

build:
	@mkdir -p bin
	go build $(LDFLAGS) -o bin/charts-mcp .

test:
	go test -race -cover ./...

test-cover:
	go test -race -coverprofile=test.out ./... && go tool cover --html=test.out

bench:
	$(eval CORES_HALF := $(shell expr `getconf _NPROCESSORS_ONLN` / 2))
	go test -parallel=$(CORES_HALF) --benchmem -benchtime=20s -bench='Benchmark.*' -run='^$$'

fmt-changed:
	@files=$$( { git diff --name-only --diff-filter=d HEAD -- '*.go'; git ls-files --others --exclude-standard -- '*.go'; } | sort -u); \
	if [ -n "$$files" ]; then \
		gofmt -w $$files; \
	fi

lint: fmt-changed
	golangci-lint run --timeout=600s
	go vet ./...

clean:
	rm -rf bin/
