BIN              := bin
TAILWIND_VERSION := 4.3.3
HTMX_VERSION     := 2.0.10
TAILWIND         := $(BIN)/tailwindcss
CSS_IN           := internal/view/static/css/input.css
CSS_OUT          := internal/view/static/css/app.css
HTMX             := internal/view/static/js/htmx.min.js
CONFIG           ?= config.yaml
VERSION          ?= $(shell git describe --tags --always --dirty)
DIST             := dist
ARCHES           := amd64 arm64

.PHONY: all tools generate css build run watch test lint fmt check clean dist

all: build

tools: $(TAILWIND) $(HTMX)
	go get -tool github.com/a-h/templ/cmd/templ

$(TAILWIND):
	mkdir -p $(BIN)
	curl -sSfL -o $@ https://github.com/tailwindlabs/tailwindcss/releases/download/v$(TAILWIND_VERSION)/tailwindcss-linux-x64
	chmod +x $@

$(HTMX):
	curl -sSfL -o $@ https://cdn.jsdelivr.net/npm/htmx.org@$(HTMX_VERSION)/dist/htmx.min.js

generate:
	go tool templ generate

css: $(TAILWIND)
	$(TAILWIND) -i $(CSS_IN) -o $(CSS_OUT) --minify

build: generate css
	CGO_ENABLED=0 go build -o $(BIN)/server ./cmd/server

run: build
	$(BIN)/server -config $(CONFIG)

watch: $(TAILWIND)
	go tool templ generate --watch & $(TAILWIND) -i $(CSS_IN) -o $(CSS_OUT) --watch

test: generate
	go test -race ./...

lint: generate
	go tool golangci-lint run

fmt:
	go tool golangci-lint fmt

check: generate css
	go tool golangci-lint fmt --diff
	go tool golangci-lint run
	go test -race ./...

dist: generate css
	rm -rf $(DIST)
	for arch in $(ARCHES); do \
		name=school-$(VERSION)-linux-$$arch; \
		mkdir -p $(DIST)/$$name; \
		CGO_ENABLED=0 GOOS=linux GOARCH=$$arch go build -trimpath -ldflags='-s -w' -o $(DIST)/$$name/server ./cmd/server || exit 1; \
		cp -r LICENSE README.md CHANGELOG.md deploy $(DIST)/$$name/; \
		tar -C $(DIST) -czf $(DIST)/$$name.tar.gz $$name; \
		rm -r $(DIST)/$$name; \
	done
	cd $(DIST) && sha256sum *.tar.gz > SHA256SUMS

clean:
	rm -rf $(BIN) $(CSS_OUT) $(DIST)
	find . -name '*_templ.go' -delete
