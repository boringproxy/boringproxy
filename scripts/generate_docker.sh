#!/bin/bash

# Run from root boringproxy folder and call with ./scripts/generate_docker.sh

############################################################
# Help                                                     #
############################################################
Help()
{
	# Display Help
	echo "Script to generate docker image for BoringProxy"
	echo "Syntax: generate_docker.sh [h|help|local|remote]"
	echo
	echo "h & help: Display help documetation"
	echo
	echo "local: Build docker image from local repo (current folder)"
	echo "options:"
	echo "  a|arch    Architecture to build for build (amd64,arm,arm64)"
	echo "  os        Operating System to build for (linux,freebsd,openbsd,windows,darwin)"
	echo "example: "
	echo "  generate_docker.sh local -a=amd -s=linux"
	echo
	echo "local: Build docker image from remote repo (Github fork)"
	echo "options:"
	echo "  a|arch    Architecture to build for build (amd64,arm,arm64)"
	echo "  os        Operating System to build for (linux,freebsd,openbsd,windows,darwin)"
	echo "  u|user    Github user"
	echo "  b|branch  Branch/Tree"
	echo "example: "
	echo "  generate_docker.sh remote -a=amd -s=linux -u=wgrobler -b=dev"
	echo
}

############################################################
############################################################
# Main program                                             #
############################################################
############################################################

# Check if file was run from correct working directory, if correct './scripts/generate_docker.sh' will exists
FILE=./scripts/generate_docker.sh
if [ ! -f "$FILE" ]; then
  echo "Script needs to be run from root boringproxy folder and call with ./scripts/generate_docker.sh"
	exit;
fi

if [ -z "$1" ];
then {
	echo "No input variabled supplied"
	echo "Here is the script help documentation:"
	echo
	Help
	exit;
}
else {
	if [ "$1" == "help" ] || [ "$1" == "h" ];
	then {
		Help
		exit;
	}
	fi
	if [ "$1" == "local" ];
	then {
		CMD='local'
		GOARCH='amd64';
		GOOS='linux';
		# Get the options
		for i in "$@"; do
  		  case $i in
					-a=*|--arch=*)
						GOARCH="${i#*=}";
						shift # past argument=value
						;;
					-os=*)
						GOOS="${i#*=}";
						shift # past argument=value
						;;
					-*|--*)
						echo "Unknown option $i"
						exit 1
						;;
					*)
						;;
				esac
			done
	}
	fi
	if [ "$1" == "remote" ];
	then {
		CMD='remote'
		GOARCH='amd64';
		GOOS='linux';
		BRANCH='master';
		REPO="github.com/boringproxy/boringproxy";
		GITHUB_USER="boringproxy"
		# Get the options
		for i in "$@"; do
  		  case $i in
					-a=*|--arch=*)
						GOARCH="${i#*=}";
						shift # past argument=value
						;;
					-os=*)
						GOOS="${i#*=}";
						shift # past argument=value
						;;
					-b=*|--branch=*)
						BRANCH="${i#*=}";
						shift # past argument=value
						;;
					-u=*|--user=*)
						GITHUB_USER="${i#*=}";
						shift # past argument=value
						;;
					-*|--*)
						echo "Unknown option $i"
						exit 1
						;;
					*)
						;;
				esac
			done
	}
	fi
}
fi

if [ "$CMD" == "local" ]; then
	DockerImage="boringproxy-$GOOS-$GOARCH"
	Dockerfile="Dockerfile"

  # Check if logo.png exists, if not create
	FILE=./default_logo.png
	if [ -f "$FILE" ]; then
		echo "$FILE exists. Using file in build";
	else
		echo "$FILE does not exist. Creating file";
		cp ./default_logo.png ./logo.png;
	fi

	# Build docker image(s)
	docker build -f "$Dockerfile" -t $DockerImage . --build-arg GOARCH=$GOARCH --build-arg GOOS=$GOOS;

fi

if [ "$CMD" == "remote" ]; then
  echo "build from remote git repo"
	DockerImage="remote-boringproxy-$GOOS-$GOARCH"
	Dockerfile="Dockerfile_remote"

	# Build docker image(s)
	REPO="https://github.com/$GITHUB_USER/boringproxy.git"
	docker build -f "$Dockerfile" -t $DockerImage . --build-arg GOARCH=$GOARCH --build-arg GOOS=$GOOS --build-arg BRANCH=$BRANCH --build-arg REPO=$REPO;
fi

# if DockerImage is set, continue
if [ -n "$DockerImage" ]; then {
	# Prune intermediate images
	docker image prune -f --filter label=boringproxy=builder

	echo
	echo "Docker file created with filename: $DockerImage"
	echo "Use $DockerImage as image name when uploading"
}
fi