#/bin/bash

version=$(git describe --tags)

rice embed-go

./scripts/build_all_platforms.sh

tar -czf ./boringproxy_${version}.tar.gz build/
