MINIREDIS_RS_BIN = ../../../target/release/miniredis-rs-server
TESTDATA = ../../testdata

.PHONY: test build

test: build $(TESTDATA)/ca.crt $(TESTDATA)/server.crt $(TESTDATA)/client.crt
	PATH="$(dir $(abspath $(MINIREDIS_RS_BIN))):$(PATH)" INT=1 go test -count=1 -v -timeout 120s ./...

build:
	cd ../../.. && cargo build --release --features tls --bin miniredis-rs-server

# Generate test TLS certificates if they don't exist.
$(TESTDATA)/ca.key:
	openssl genrsa -out $@ 2048

$(TESTDATA)/ca.crt: $(TESTDATA)/ca.key
	openssl req -new -x509 -key $< -out $@ -days 3650 -subj "/CN=Test CA" \
		-addext "basicConstraints=critical,CA:TRUE"

$(TESTDATA)/server.key:
	openssl genrsa -out $@ 2048

$(TESTDATA)/server.crt: $(TESTDATA)/server.key $(TESTDATA)/ca.crt $(TESTDATA)/ca.key
	openssl req -new -key $(TESTDATA)/server.key -subj "/CN=Server" \
		-addext "subjectAltName=DNS:Server" | \
	openssl x509 -req -CA $(TESTDATA)/ca.crt -CAkey $(TESTDATA)/ca.key \
		-CAcreateserial -out $@ -days 3650 \
		-copy_extensions copyall

$(TESTDATA)/client.key:
	openssl genrsa -out $@ 2048

$(TESTDATA)/client.crt: $(TESTDATA)/client.key $(TESTDATA)/ca.crt $(TESTDATA)/ca.key
	openssl req -new -key $(TESTDATA)/client.key -subj "/CN=Client" | \
	openssl x509 -req -CA $(TESTDATA)/ca.crt -CAkey $(TESTDATA)/ca.key \
		-CAcreateserial -out $@ -days 3650
