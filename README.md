# Disclaimer

boringproxy is currently beta-quality software. While I am a big believer in
open source, my primary goal at the moment is to build a sustainable
business around the code I write. So for the most part I can only afford to
spend time fixing problems that arise in my own usage of boringproxy. That
said, feel free to create
[GitHub issues](https://github.com/boringproxy/boringproxy/issues)
and I'll try to help as I have time.

# What is it?

If you have a webserver running on one computer (say your development laptop),
and you want to expose it securely (ie HTTPS) via a public URL, boringproxy
allows you to easily do that.

**NOTE:** For information on downloading and running boringproxy, it's best to
start on the website, [boringproxy.io](https://boringproxy.io/). The information
in this README is just for building from source.


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

Make the logo image file. It gets baked into the executable so it needs to
be available at build time. Note that you don't have to use the official
logo for the build. Any PNG will do. It's currently just used for the favicon.

```bash
./scripts/generate_logo.sh
```

```bash
cd cmd/boringproxy
go build
```

Give the executable permission to bind low ports (ie 80/443):

```bash
sudo setcap cap_net_bind_service=+ep boringproxy
```

# Running

## Server

```bash
./boringproxy server
```

## Client

```bash
./boringproxy client -server bpdemo.brng.pro -token fKFIjefKDFLEFijKDFJKELJF -client-name demo-client -user demo-user
```
