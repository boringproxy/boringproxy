boringproxy development is sponsored by [TakingNames.io](https://takingnames.io).
boringproxy offers full integration with TakingNames.io, providing the simplest
way to get up and running with your own domain. More information [here](https://takingnames.io/blog/introducing-takingnames-io),
and a demo video of boringproxy working with TakingNames.io [here](https://youtu.be/9hf72-fYTts).

<a href='https://takingnames.io/blog/introducing-takingnames-io'>
  <img src='https://user-images.githubusercontent.com/7820200/148330003-5f8062ff-22b2-423d-b945-3db87abf10e5.png' width='400'></img>
</a>


# What is boringproxy?

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

To build with version information:
```bash
go build -ldflags "-X main.Version=$(git describe --tags)"
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

# Getting Help

If you run into problems running boringproxy, the best place to ask for help is
over at the [IndieBits][0] community, where we have a [dedicated section][1]
for boringproxy support. If you think you've found a bug, or want to discuss
development, please [open an issue][2].

[0]: https://forum.indiebits.io

[1]: https://forum.indiebits.io/c/boringproxy-support/9

[2]: https://github.com/boringproxy/boringproxy/issues
