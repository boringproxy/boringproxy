# Planned features

* Community forum
* GUI client
* Custom SSH keys
* Auto-DNS
  * DNS verification before creating tunnels. Necessary for secure multi-user
    setups.
  * libdns integration
  * Add 3rd-party tokens for controlling DNS
  * Maybe add a DNS/Domains page and require users to add domains there before
    they can use them for tunnels. This creates a natural place to explain what
    is wrong when domain stuff breaks.
* Built-in static file hosting
  * Client determines which directories are exposed
* Password-protected tunnels
* TCP tunnels
  * Allow client to specify binding to addresses other than 127.0.0.1.
* IPv6


# Potential features

* Built-in GemDrive
  * Allows web UI to browse files on the clients
* WireGuard hub
* Create tunnels by full URL; not just domains. Allows things like sharing
  specific files and having multiple servers behind a single domain.
* Allow multiple upstreams for load balancing/HA.


# Tunnel variations

* Plain TCP
* SSH with custom keys
* SSH with server-generated keys
* Future protocols
  * Custom SSH?
  * Custom TLS?
  * Custom QUIC?
* Which client?
* Which client port?
