# Files to run server using docker

## Update compose file

Edit docker-compose.yml and change the following under **commands** for service **boringproxy**
- bp.example.com: your admin domain

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