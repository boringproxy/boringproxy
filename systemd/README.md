# Installing Service

## Server

The folling steps assume that boringproxy is already installed on the server. If you haven't installed the server, follow [installation](https://boringproxy.io/installation/) instructions in the documentation.

boringproxy needs to be installed in **/usr/local/bin/boringproxy** for the default service file to work, this location can be changed in the service file

### Create boringproxy user & group
The service will be run as user 'boringproxy'. Runnning the service as root is not recomended. 

Add user "boringproxy"
```bash
useradd -M boringproxy
```

Add group "boringproxy"
```bash
groupadd boringproxy;
```

Add user "boringproxy" to group "boringproxy"
```bash
usermod -a -G boringproxy boringproxy
```

### Download & edit service file

Copy service file from GitHub
```bash
wget https://raw.githubusercontent.com/WGrobler/boringproxy/master/systemd/boringproxy-server.service
```

Edit service file to include your setup information

Default working directory is ***"/opt/boringproxy/"***, you can change this in the service file to another directory. 
Make sure the directory exists, otherwise create WorkingDirectory
```bash
mkdir -p /opt/boringproxy/
```

Default location for your boringproxy executable file is ***"/usr/local/bin/boringproxy"***, you can change this in the service file to another path.
Make sure the file exists, otherwise move file from the current directory to ***"/usr/local/bin/boringproxy"***
```bash
mv ./boringproxy /usr/local/bin/boringproxy
```

Reload the service files to include the new service.
```bash
systemctl daemon-reload
```

Start your service
```bash
systemctl start boringproxy-server.service
```

To check the status of your service
```bash
systemctl status boringproxy-server.service
```

To enable your service on every reboot
```bash
systemctl enable boringproxy-server.service
```

To disable your service on every reboot
```bash
systemctl disable  boringproxy-server.service
```

## Client

```bash
./boringproxy client -server bpdemo.brng.pro -token fKFIjefKDFLEFijKDFJKELJF -client-name demo-client -user demo-user
```
