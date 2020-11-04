#/bin/bash

version=$(git describe --tags)

./scripts/build_all_platforms.sh

tar -czf ./boringproxy_${version}.tar.gz build/
