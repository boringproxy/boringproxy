# What is it?

If you have a webserver running on one computer (say your development laptop),
and you want to expose it securely (ie HTTPS) via a public URL, boringproxy
allows you to easily do that.

You can learn more at [boringproxy.io](https://boringproxy.io/).


# Building

```bash
git clone https://github.com/boringproxy/boringproxy
```

```bash
cd boringproxy
```

If you don't already have golang installed:

```bash
./install_go.sh
source $HOME/.bashrc
```

```bash
go build
```

To embed the web UI into the executable:

```bash
go get github.com/GeertJohan/go.rice/rice
rice embed-go
go build
```

# Running

## Server

```bash
boringproxy server -admin-domain bpdemo.brng.pro
```

## Client

```bash
boringproxy client -server bpdemo.brng.pro -token fKFIjefKDFLEFijKDFJKELJF -client-name demo-client -user demo-user
```
