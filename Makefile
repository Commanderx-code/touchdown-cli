.PHONY: build run test install

PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin

build:
	mkdir -p bin
	go build -o bin/touchdown .

run:
	go run .

test:
	go test ./...

install: build
	install -Dm755 bin/touchdown $(BINDIR)/touchdown
	@if [ -e "$(BINDIR)/td" ] || [ -L "$(BINDIR)/td" ]; then \
		[ "$$(readlink "$(BINDIR)/td")" = "touchdown" ] || { echo "$(BINDIR)/td already exists and is not Touchdown" >&2; exit 1; }; \
	else ln -s touchdown "$(BINDIR)/td"; fi
