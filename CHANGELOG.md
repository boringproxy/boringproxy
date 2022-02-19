# v0.9.0

* Raw TLS tunnels implemented, which adds WebSockets support.
* Improved security of tokens. They can now be limited to only work for
  specific clients.
* A default logo is included in the repo, so inkscape is no longer required to
  build the project (thanks @WGrobler!).
* Docker instructions, scripts, and examples greatly improved (thanks
  @WGRobler!)
* Added IPv6 support.
* API simplified so client doesn't need to be run with `-user` or
  `-client-name` if that information can be extracted from the token.
* Added `-acme-use-staging` to allow use of Let's Encrypt staging servers.
* Added page to allow managing clients from the web UI. Previously they were
  silently added when the client first connected.
* Added `-behind-proxy` flag so X-Forwarded-For header is only added when the
  flag is set. This improves security so clients can't spoof their IPs.


# v0.8.2

* Integration with [TakingNames.io](https://takingnames.io).
* Support now available through the [IndieBits forum](https://forum.indiebits.io/).
* Switch to more traditional HTML UI. Was doing some cool but hacky CSS stuff.
* Replaced go.rice with embed from stdlib.
* Check if ports are publicly accessible on startup.
* Add individual pages to look at tunnel details.
* Implement support for unencrypted HTTP.
* Can now select server HTTP/HTTPS ports.
* Add Forwarded and X-Forwarded-For proxy headers.
* Implement printing login link as QR code on the command line.


# v0.7.0

* Fixed server authorized_key file getting huge.
* Added FreeBSD and OpenBSD builds.
* Fix redirects on client-terminated tunnels.


# v0.6.0

* Various internal improvements, especially to make boringproxy easier to use as a library in other programs.
* Renamed amd64 to x86_64 to be easier to distinguish from arm64.
* Allow tunnel port to be selected, allowing boringproxy to more easily be used like a normal reverse proxy.
* Various other small bug fixes and UX improvements.


# v0.5.0

* Improved UX
  * Print usage information (thanks @arp242!)
  * Some better error messages
* Added systemd docs and examples (thanks @voidrot!)
* Move main package into cmd/boringproxy so server and client can be imported into other programs.
* Stream requests. Server was reading entire requests before forwarding to upstream (similar to nginx default). Now streams everything.
