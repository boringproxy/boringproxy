FROM ubuntu:16.04

RUN apt-get update && apt-get install -y curl git

RUN git clone https://github.com/boringproxy/boringproxy
WORKDIR boringproxy
RUN ./install_go.sh
ENV PATH="${PATH}:/usr/local/go/bin"
ENV PATH="${PATH}:/root/go/bin"
RUN go get github.com/GeertJohan/go.rice/rice
RUN rice embed-go
RUN go build

CMD ./boringproxy server
