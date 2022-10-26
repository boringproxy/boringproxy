#!/bin/bash

export VERSION=1.19 OS=linux ARCH=amd64

curl -O https://dl.google.com/go/go$VERSION.$OS-$ARCH.tar.gz
tar -C /usr/local -xzvf go$VERSION.$OS-$ARCH.tar.gz
rm go$VERSION.$OS-$ARCH.tar.gz

echo 'export PATH=$PATH:/usr/local/go/bin' >> $HOME/.bashrc
