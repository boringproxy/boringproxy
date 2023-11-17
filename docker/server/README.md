# Files to run server using docker

## Update compose file

Edit docker-compose.yml and change the following under **commands** for service **boringproxy**
- bp.example.com: your admin domain
- your-email-address: the email address to register with Let's Encrypt

***Since the -accept-ca-terms flag is set in the compose file, this will automatically accept terms and conditions of Let's Encrypt.***

## Build image from source and run server in docker
You can build the image from source. This requires that you clone the GitHub repo and start docker using the compose command below:

```bash
docker-compose -f docker-compose.yml -f source.yml up -d
```

## Download prebuild image and run server in docker
If you don't want to build the image, a prebuild image can be downloaded from GitHub. Start docker using the compose commands below to download the image and start the container.

```bash
docker-compose -f docker-compose.yml -f prebuild.yml up -d
```

Once it's running, the GUI can be accessed at the admin domain you specified. It will ask for an access token. The token can be found by accessing boringproxy_db.json:
```bash
nano /var/lib/docker/volumes/server_storage/_data/boringproxy_db.json
```
