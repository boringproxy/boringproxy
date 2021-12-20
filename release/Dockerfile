# This Dockerfile is for building releases. It creates executables files for
# all supported platforms. It intentially uses an old version of Linux so that
# the produced executables will run on old versions.

FROM ubuntu:16.04

RUN apt-get update && apt-get install -y curl git inkscape

RUN git clone https://github.com/boringproxy/boringproxy
WORKDIR boringproxy
RUN ./scripts/install_go.sh
ENV PATH="${PATH}:/usr/local/go/bin"
ENV PATH="${PATH}:/root/go/bin"
