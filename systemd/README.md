# Installing Service

## Server

The folling steps assume that boringproxy is already installed on the server. If you haven't installed the server, follow [installation](https://boringproxy.io/installation/) instructions in the documentation.

boringproxy needs to be installed in **/usr/local/bin/boringproxy** for the default service file to work, this location can be changed in the service file

### Create boringproxy user & group
The service will be run as user 'boringproxy'. Runnning the service as root is not recomended. 

Add user "boringproxy"
```bash
useradd -s /bin/bash -d /home/boringproxy/ -m boringproxy;
```

Add group "boringproxy"
```bash
groupadd boringproxy;
```

Add user "boringproxy" to group "boringproxy"
```bash
usermod -a -G boringproxy boringproxy
```

Create SSH folder for user. BoringProxy assumes the folder already exists. If it does not exist, the program will fail to add tunnels.
```bash
mkdir /home/boringproxy/.ssh
chown boringproxy:boringproxy /home/boringproxy/.ssh
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

### Install service file to systemd

Copy service file to "/etc/systemd/system/"
```bash
mv ./boringproxy-server.service /etc/systemd/system/
```
Reload the service files to include the new service.
```bash
systemctl daemon-reload
```

### Manual start (once off only)
When boringproxy start for the first time, it requires a manual input of your email address. This email address will be used when registering Certificates with Let's Encrypt.
By stating the server manually, you can enter the required information and ensure the server is starting correctly under the new user. 

To start the server, you will need to change the current directory to your WorkingDirectory (as indicated in your service file) and then run the ExecStart command (as indicated in your service file). If you made changes to the default WorkingDirectory or boringproxy executable file path, change the command below accordingly. 
Change the admin-domain in the command below to your admin-domain and enter your email address when prompted
```bash
runuser -l boringproxy -c 'cd /opt/boringproxy; /usr/local/bin/boringproxy server -admin-domain bp.example.com'
```

If your server was successfully started, close the running process and start it again using the service. Since the process was started as a different user, you will have to kill the foreground process (***Ctrl + C***) as well as close the process started as user boringproxy. To kill all running processes for user boringproxy, use the command below:
```bash
pkill -u boringproxy
```

To check if boringproxy is still running, you can look if a process is listening on port 443 using:
```bash
netstat -tulpn | grep LISTEN | grep 443
```
If nothing is returned, no process is currently using port 443. Alternatively you will receive a result like:
*tcp6       0      0 :::443                  :::*                    LISTEN      9461/boringproxy*

### Service commands

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
