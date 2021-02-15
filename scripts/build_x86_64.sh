#!/bin/bash

echo Building platform $1-x86_64
CGO_ENABLED=0 GOOS=$1 GOARCH=amd64 go build -o build/boringproxy-$1-x86_64
