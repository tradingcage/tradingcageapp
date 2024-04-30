#!/usr/bin/env bash

set -ex

cd frontend
npm install
./svelte-build.sh
cd ..

go build main.go