# bv Makefile
#
# Build with SQLite FTS5 (full-text search) support enabled

.PHONY: build build-all install install-all clean test

# Enable FTS5 for full-text search in SQLite exports
export CGO_CFLAGS := -DSQLITE_ENABLE_FTS5

build:
	go build -o bv ./cmd/bv

build-ack:
	go build -o bd-ack ./cmd/bd-ack

build-all: build build-ack

install:
	go install ./cmd/bv

install-ack:
	go install ./cmd/bd-ack

install-all: install install-ack

clean:
	rm -f bv bd-ack
	go clean

test:
	go test ./...
