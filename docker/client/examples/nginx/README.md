# FUse boringproxy with nginx

## Update compose file

Edit docker-compose.yml and change the following under **commands** for service **boringproxy**
- bp.example.com: your admin domain
- your-user-token: token generated by your server
- your-email-address: the email address to register with Let's Encrypt


## Add tunnel in WebUI

Add new tunnel with the following config

- Domain: domain for this tunnel
- Tunnel Type: **Client TSL**
- Tunnel Port: **Random**
- Client Name: **docker-nginx**
- Client Address: **nginx**
- Client Port: **8123**

## Start containers
To start the container(s), run the start script in the example folder
```bash
./start.sh
```