#!/usr/bin/env bash

set -e
set -x

go mod download

cd cli/daemon/dash/dashapp
npm install
node node_modules/esbuild/install.js
npm run build

