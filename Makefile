BIN              := bin
TAILWIND_VERSION := 4.3.3
HTMX_VERSION     := 2.0.10
TAILWIND         := $(BIN)/tailwindcss
CSS_IN           := internal/view/static/css/input.css
CSS_OUT          := internal/view/static/css/app.css
HTMX             := internal/view/static/js/htmx.min.js
CONFIG           ?= config.yaml

.PHONY: all tools generate css build run watch test lint fmt check clean

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

clean:
	rm -rf $(BIN) $(CSS_OUT)
	find . -name '*_templ.go' -delete
