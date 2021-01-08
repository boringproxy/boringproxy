#!/bin/bash

version=$(git describe --tags)

./scripts/generate_logo.sh

rice embed-go

cd ./cmd/boringproxy

../../scripts/build_arch.sh linux 386
../../scripts/build_arch.sh linux amd64
../../scripts/build_arch.sh linux arm
../../scripts/build_arch.sh linux arm64
../../scripts/build_arch.sh windows 386 .exe
../../scripts/build_arch.sh windows amd64 .exe
../../scripts/build_arch.sh darwin amd64

mv build ../../
cd ../../

tar -czf ./boringproxy_${version}.tar.gz build/
