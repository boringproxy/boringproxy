#/bin/bash

version=$(git describe --tags)

function buildArch {
    echo Building platform $1-$2
    GOOS=$1 GOARCH=$2 go build -o build/boringproxy-$1-$2$3
}

./scripts/generate_logo.sh

rice embed-go

buildArch linux amd64
buildArch linux arm
buildArch linux arm64

buildArch windows amd64 .exe

buildArch darwin amd64

./scripts/build_all_platforms.sh

tar -czf ./boringproxy_${version}.tar.gz build/
