.PHONY: build run test clean install release

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

build:
	go build -ldflags '$(LDFLAGS)' -o proxfy .

run: build
	./proxfy start

install: build
	sudo cp proxfy /usr/local/bin/proxfy
	@echo "✅ proxfy installed to /usr/local/bin/proxfy"

clean:
	rm -f proxfy
	go clean

test:
	go vet ./...
	go test ./... -v

release-dry:
	goreleaser release --snapshot --clean

release:
	@echo "To release, create and push a tag:"
	@echo "  git tag v0.1.0"
	@echo "  git push origin v0.1.0"
	@echo "GitHub Actions will handle the rest."
