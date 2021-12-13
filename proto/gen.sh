#!/usr/bin/env bash

set -e -x

protoc -I . --go_out=. --go_opt=paths=source_relative ./encore/parser/meta/v1/meta.proto
protoc -I . --go_out=. --go_opt=paths=source_relative ./encore/parser/schema/v1/schema.proto
protoc -I . --go_out=. --go_opt=paths=source_relative ./encore/engine/trace/trace.proto
protoc -I . --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative ./encore/daemon/daemon.proto
