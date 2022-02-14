#!/bin/bash

# This file is used to upload a build docker image to GitHub.
# Run build_docker.sh first to create new image
# Run from root boringproxy folder and call with ./scripts/upload_docker_image.sh github-username
# github-username must be lowercase

# https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry

if [ -z "$1" ];
then {
	echo "Container name required";
	exit;
}
fi

if [ -z "$2" ];
then echo "GitHub username required";
else {
	if [ -z "$3" ];
	then {
		echo "No TAG set, using latest";
		tag='latest';
	}
	else tag=$3;
	fi
	docker image tag $1 ghcr.io/$2/$1:$tag
	CR_PAT=`cat ~/.auth_tokens/github`
	echo $CR_PAT | docker login ghcr.io -u $2 --password-stdin
	docker push ghcr.io/$2/$1:$tag
} fi

