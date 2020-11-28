#!/bin/bash

echo Building platform $1-$2
CGO_ENABLED=0 GOOS=$1 GOARCH=$2 go build -o build/boringproxy-$1-$2$3
