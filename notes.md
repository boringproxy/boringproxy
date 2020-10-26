# Planned features

* Community forum
* GUI client
* Auto-DNS
  * DNS verification before creating tunnels. Necessary for secure multi-user
    setups.
    * Rely on CNAMEs (ie username.boringproxy.io) or TXT records (ie
      boringproxy-account=user@example.com)?
  * libdns integration
  * Add 3rd-party tokens for controlling DNS
  * Maybe add a DNS/Domains page and require users to add domains there before
    they can use them for tunnels. This creates a natural place to explain what
    is wrong when domain stuff breaks.
* Built-in static file hosting
  * Client determines which directories are exposed
* IPv6


# Potential features

* Built-in GemDrive
  * Allows web UI to browse files on the clients
* WireGuard hub
* Create tunnels by full URL; not just domains. Allows things like sharing
  specific files and having multiple servers behind a single domain.
* Allow multiple upstreams for load balancing/HA.
* Custom SSH keys
  * Partially implemented but commented out. It's tricky to manage them,
    especially using the authorized_keys file. I think a lot of use cases are
    handled by allowing the key for each tunnel to be downloaded manually,
    which is already implemented.


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
