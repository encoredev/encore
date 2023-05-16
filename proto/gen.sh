#!/usr/bin/env bash

set -e -x

GO_OPT=paths=source_relative
GRPC_OPT=paths=source_relative

TS_PROTO_PATH="../cli/daemon/dash/dashapp/node_modules/.bin/protoc-gen-ts_proto"
TS_OPT=outputClientImpl=false,outputEncodeMethods=false,outputJsonMethods=false,snakeToCamel=false,stringEnums=true,fileSuffix=.pb,useOptionals=fakeValueToOnlyAllowUnionForUndefinedOnOptionalTypes

protoc -I . --go_out=. --go_opt=$GO_OPT --plugin=$TS_PROTO_PATH --ts_proto_out=. --ts_proto_opt=$TS_OPT \
  ./encore/parser/meta/v1/meta.proto

protoc -I . --go_out=. --go_opt=$GO_OPT --plugin=$TS_PROTO_PATH --ts_proto_out=. --ts_proto_opt=$TS_OPT \
  ./encore/parser/schema/v1/schema.proto

protoc -I . --go_out=. --go_opt=$GO_OPT \
  ./encore/engine/trace/trace.proto

protoc -I . --go_out=. --go_opt=$GO_OPT \
  ./encore/engine/trace2/trace2.proto

protoc -I . --go_out=. --go_opt=$GO_OPT --go-grpc_out=. --go-grpc_opt=$GRPC_OPT \
  ./encore/daemon/daemon.proto
