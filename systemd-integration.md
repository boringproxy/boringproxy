# Systemd Integration

These instructions assume that you have followed the [Installation instruction](https://boringproxy.io/#installation) and installed the boringproxy binary to `/usr/local/bin/`

If you install the binary to a different path you will need to update the path in the service files.

---

## BoringProxy Server Service
This service file along with the assumptions above assumes that you will run the initial launch of the server from the `/root` directory as the `root` user.

Download the boringproxy-server.service file
```bash
# with wget
sudo wget https://raw.githubusercontent.com/boringproxy/boringproxy/master/scripts/boringproxy-server.service -O /etc/systemd/system/boringproxy-server.service

# or with curl
sudo curl https://raw.githubusercontent.com/boringproxy/boringproxy/master/scripts/build.sh --output /etc/systemd/system/boringproxy-server.service
```

Edit `/etc/systemd/system/boringproxy-server.service` and replace the admin domain `bp.example.com` with the domain that the server will be available at. EX: `-admin-domain proxy.bpuser.me`

Enable and start the boringproxy server service with the following command
```bash
sudo systemctl enable --now boringproxy-server.service
```

This will make sure that boringproxy server will always start backup if the host is restarted.

---

## BoringProxy Client Service

Download the boringproxy-client@.service file
```bash
# with wget
sudo wget https://raw.githubusercontent.com/boringproxy/boringproxy/master/scripts/boringproxy-client%40.service -O "/etc/systemd/system/boringproxy-client@.service"

# or with curl
sudo curl https://raw.githubusercontent.com/boringproxy/boringproxy/master/scripts/boringproxy-client%40.service --output "/etc/systemd/system/boringproxy-client@.service"
```

Edit `/etc/systemd/system/boringproxy-client@.service` and replace the server address `bp.example.com` with the domain that the server is located at. EX: `-server proxy.bpuser.me`

also edit the token value `your-bp-server-token` with the token from when you installed the server. EX: `-token rt42g.......3fn`

Enable and start the boringproxy server service with the following command
```bash
# the value after the @ symbol in the service name is what will determine the name of the client in the Admin UI
sudo systemctl enable --now boringproxy-client@default.service
```

This will make sure that boringproxy client will always start backup and reconnect to the boringclient server if the host is restarted or goes down for some reason.

### Notes

This systemd service file is a template service which allows you to spawn multiple clients with a specified name. 

If you do not need/want the ability to launch multiple clients with a single service file and do not want to have to specify `boringproxy-client@<client-name>.service` when interacting with the service, rename the service file to `boringproxy-client.service` and remove the `%I` from the `Description` field and replace the `%i` after `-client-name` with the name you want the client to have. after those modifications you can use the service as `boringproxy-client.service` 

---

## Using a user other than root

As good practice tells us we really should not run services as the root user. If you would like to follow these best practices the service file `User`, `Group`, and `WorkingDirectory` values will need to be updated.

The below commands will setup and fix the service file to use a user besides root
```bash
# create the system users homedir - This is needed as you need a place to store the `boringproxy_db.json` file
sudo mkdir -pv /opt/boringproxy

# create the system user - We are using a system user as we don't want regular user permissions assigned since all it is going to be doing is running boringproxy for us. We also specify the shell as /bin/false so that nothing can login as this user just incase.
sudo useradd -d /opt/boringproxy -m --system --shell /bin/false boringproxy

# If you have already launched the boringproxy server you need to move the db file to keep your settings
# mv /root/boringproxy_db.json /opt/boringproxy/boringproxy_db.json

# Set the permissions just to make sure especially if you have moved the db file into the directory.
sudo chown boringproxy:boringproxy -R /opt/boringproxy
# You can also lock down the directory even further with the following if your somewhat paranoid
# chmod 700 /opt/boringproxy

# update the service files to use the new user, group, and user home directory
# Server Service
sed -i "s/^User=.*/User=boringproxy/" /etc/systemd/system/boringproxy-server.service
sed -i "s/^Group=.*/Group=boringproxy/" /etc/systemd/system/boringproxy-server.service
sed -i "s/^WorkingDirectory=.*/WorkingDirectory=/opt/boringproxy/" /etc/systemd/system/boringproxy-server.service
# Client Service
sed -i "s/^User=.*/User=boringproxy/" /etc/systemd/system/boringproxy-client\@.service
sed -i "s/^Group=.*/Group=boringproxy/" /etc/systemd/system/boringproxy-client\@.service
```