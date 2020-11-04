#!/bin/bash

function buildArch {
    echo Building platform $1-$2
    GOOS=$1 GOARCH=$2 go build -o build/boringproxy-$1-$2$3
}

buildArch linux amd64
buildArch linux arm
buildArch linux arm64

buildArch windows amd64 .exe

buildArch darwin amd64
