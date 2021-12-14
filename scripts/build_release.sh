#!/bin/bash

version=$(git describe --tags)

./scripts/generate_logo.sh

cd ./cmd/boringproxy

../../scripts/build_x86_64.sh linux
../../scripts/build_x86_64.sh freebsd
../../scripts/build_x86_64.sh openbsd
../../scripts/build_x86_64.sh windows .exe
../../scripts/build_x86_64.sh darwin

../../scripts/build_arch.sh linux 386
../../scripts/build_arch.sh linux arm
../../scripts/build_arch.sh linux arm64
../../scripts/build_arch.sh freebsd 386
../../scripts/build_arch.sh freebsd arm
../../scripts/build_arch.sh freebsd arm64
../../scripts/build_arch.sh openbsd 386
../../scripts/build_arch.sh openbsd arm
../../scripts/build_arch.sh openbsd arm64
../../scripts/build_arch.sh windows 386 .exe

mv build ../../
cd ../../

tar -czf ./boringproxy_${version}.tar.gz build/
