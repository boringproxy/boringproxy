#/bin/bash

version=$(git describe --tags)

function buildArch {
    echo Building platform $1-$2
    GOOS=$1 GOARCH=$2 go build -o build/boringproxy-$1-$2$3
}

./scripts/generate_logo.sh

rice embed-go

buildArch linux 386
buildArch linux amd64
buildArch linux arm
buildArch linux arm64

./scripts/build_android.sh

buildArch windows 386 .exe
buildArch windows amd64 .exe

buildArch darwin amd64

tar -czf ./boringproxy_${version}.tar.gz build/
