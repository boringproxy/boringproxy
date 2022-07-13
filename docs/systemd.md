# Systemd Integration

These instructions assume that you have followed the [Installation instruction](https://boringproxy.io/#installation) and installed the boringproxy binary to `/usr/local/bin/`

If you install the binary to a different path you will need to update the path in the service files.

---

## System User and WorkingDirectory Setup

The following steps setup a user and working directory for boringproxy to match with standard best practices as not running processes as the root user.

### Admin Server & Client Setup
Currently the boringproxy client does not need 

```bash
# create the system user - We are using a system user as we don't want regular user permissions assigned since all it is going to be doing is running boringproxy for us. We also specify the shell as /bin/false so that nothing can login as this user just incase.
sudo useradd -d /opt/boringproxy -m --system --shell /bin/false boringproxy

# Since the boringproxy working directory houses data that we dont want to be exposed to other services/users are all we will make it so that ony the boringproxy user itself us able to access files and directories in the working directory
sudo chmod 700 /opt/boringproxy
```

## boringproxy Server Service

Download the boringproxy-server.service file
```bash
# with wget
wget https://raw.githubusercontent.com/boringproxy/boringproxy/master/systemd/boringproxy-server.service -O /tmp/boringproxy-server.service

# or with curl
curl https://raw.githubusercontent.com/boringproxy/boringproxy/master/systemd/boringproxy-server.service --output /tmp/boringproxy-server.service

# move the systemd file into the correct location
sudo mv /tmp/boringproxy-server.service /etc/systemd/system/boringproxy-server.service
```


Edit `/etc/systemd/system/boringproxy-server.service` and replace the admin domain `bp.example.com` with the domain that the server will be available at. EX: `-admin-domain proxy.bpuser.me`

Enable and start the boringproxy server service with the following command
```bash
sudo systemctl enable --now boringproxy-server.service
```

This will make sure that boringproxy server will always start backup if the host is restarted.

---

## boringproxy Client Service

Download the boringproxy-client@.service file
```bash
# with wget
wget https://raw.githubusercontent.com/boringproxy/boringproxy/master/systemd/boringproxy-client.service -O "/tmp/boringproxy-client@.service"

# or with curl
curl https://raw.githubusercontent.com/boringproxy/boringproxy/master/systemd/boringproxy-client.service --output "/tmp/boringproxy-client@.service"

sudo mv /tmp/boringproxy-client@.service /etc/systemd/system/boringproxy-client@.service
```

Edit `/etc/systemd/system/boringproxy-client@.service` and replace the server address `bp.example.com` with the domain that the server is located at. EX: `-server proxy.bpuser.me`

also edit the token value `your-bp-server-token` with the token from when you installed the server. EX: `-token rt42g.......3fn`

Enable and start the boringproxy server service with the following command
```bash
# the value after the @ symbol in the service name is what will determine the name of the client in the Admin UI
sudo systemctl enable --now boringproxy-client@default.service
```

This will make sure that boringproxy client will always start backup and reconnect to the boringclient server if the host is restarted or goes down for some reason.

## Notes
### Updating an existing boringproxy Server instance
If you have already ran the admin server you will need to migrate the db and change its permissions to keep your existing settings.

```bash
sudo mv /root/boringproxy_db.json /opt/boringproxy/boringproxy_db.json
sudo chown boringproxy:boringproxy /opt/boringproxy/boringproxy_db.json
```

### Client Service Unit File
This systemd service file is a template service which allows you to spawn multiple clients with a specified name. 

If you do not need/want the ability to launch multiple clients with a single service file and do not want to have to specify `boringproxy-client@<client-name>.service` when interacting with the service, rename the service file to `boringproxy-client.service` and remove the `%I` from the `Description` field and replace the `%i` after `-client-name` with the name you want the client to have. after those modifications you can use the service as `boringproxy-client.service` 
