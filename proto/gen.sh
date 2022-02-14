#!/usr/bin/env bash

set -e -x

TS_PROTO_PATH="../cli/daemon/dash/dashapp/node_modules/.bin/protoc-gen-ts_proto"

protoc -I . \
  --plugin=$TS_PROTO_PATH --ts_proto_out=. --ts_proto_opt=outputClientImpl=false --ts_proto_opt=outputEncodeMethods=false \
  --ts_proto_opt=outputJsonMethods=false --ts_proto_opt=snakeToCamel=false --ts_proto_opt=stringEnums=true \
  --ts_proto_opt=fileSuffix=.pb --ts_proto_opt=useOptionals=fakeValueToOnlyAllowUnionForUndefinedOnOptionalTypes \
  --go_out=. --go_opt=paths=source_relative \
  ./encore/parser/meta/v1/meta.proto

protoc -I . \
  --plugin=$TS_PROTO_PATH --ts_proto_out=. --ts_proto_opt=outputClientImpl=false --ts_proto_opt=outputEncodeMethods=false \
    --ts_proto_opt=outputJsonMethods=false --ts_proto_opt=snakeToCamel=false --ts_proto_opt=stringEnums=true \
    --ts_proto_opt=fileSuffix=.pb --ts_proto_opt=useOptionals=fakeValueToOnlyAllowUnionForUndefinedOnOptionalTypes \
  --go_out=. --go_opt=paths=source_relative \
  ./encore/parser/schema/v1/schema.proto

protoc -I . --go_out=. --go_opt=paths=source_relative ./encore/engine/trace/trace.proto
protoc -I . --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative ./encore/daemon/daemon.proto
