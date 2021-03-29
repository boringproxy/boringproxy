FROM golang:1.15-alpine3.12 as builder

WORKDIR /build

RUN apk add git
RUN go get github.com/GeertJohan/go.rice/rice

COPY go.* ./
RUN go mod download
COPY . .

RUN rice embed-go
RUN CGO_ENABLED=0 go build -o boringproxy
RUN chmod +x boringproxy
FROM scratch
EXPOSE 80 443

COPY --from=builder /build/boringproxy /

ENTRYPOINT ["/boringproxy"]
CMD ["server"]
