MINIREDIS_RS_BIN = ../../../target/release/miniredis-rs-server

.PHONY: test build

test: build
	PATH="$(dir $(abspath $(MINIREDIS_RS_BIN))):$(PATH)" INT=1 go test -count=1 -v -timeout 120s ./...

build:
	cd ../../.. && cargo build --release --features tls --bin miniredis-rs-server
