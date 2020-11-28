#!/bin/bash

echo Building platform android-arm
GOOS=android GOARCH=arm CGO_ENABLED=1 CC=$HOME/Android/Sdk/ndk/21.3.6528147/toolchains/llvm/prebuilt/linux-x86_64/bin/armv7a-linux-androideabi30-clang go build -o build/boringproxy-android-arm

echo Building platform android-arm64
GOOS=android GOARCH=arm64 CGO_ENABLED=1 CC=$HOME/Android/Sdk/ndk/21.3.6528147/toolchains/llvm/prebuilt/linux-x86_64/bin/aarch64-linux-android30-clang go build -o build/boringproxy-android-arm64
