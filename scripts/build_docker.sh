#!/bin/bash

# Run from root boringproxy folder and call with ./scripts/build_docker.sh

############################################################
# Help                                                     #
############################################################
Help()
{
	# Display Help
	echo "Script to buid BoringProxy executables using docker"
	echo "Syntax: build_docker.sh [h|help|local|remote]"
	echo
	echo "h & help: Display help documetation"
	echo
	echo "local: Build executables from local repo (current folder)"
	echo "options:"
	echo "  a|arch    Architecture to build for build (amd64,arm,arm64)"
	echo "  os        Operating System to build for (linux,freebsd,openbsd,windows,darwin)"
	echo "example: "
	echo "  build_docker.sh local a=amd s=linux"
	echo
	echo "local: Build executables remote repo (Github fork)"
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

# Check if file was run from correct working directory, if correct './scripts/build_docker.sh' will exists
FILE=./scripts/build_docker.sh
if [ ! -f "$FILE" ]; then
  echo "Script needs to be run from root boringproxy folder and call with ./scripts/build_docker.sh"
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
						shift;
						;;
					-os=*)
						GOOS="${i#*=}";
						shift;
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
		GITHUB_USER="boringproxy"
		# Get the options
		for i in "$@"; do
  		  case $i in
					-a=*|--arch=*)
						GOARCH="${i#*=}";
						shift;
						;;
					-os=*)
						GOOS="${i#*=}";
						shift;
						;;
					-b=*|--branch=*)
						BRANCH="${i#*=}";
						shift;
						;;
					-u=*|--user=*)
						GITHUB_USER="${i#*=}";
						shift;
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

# Get current timestamp and set at TAG
timestamp=$(date +%s)

# Make build folder if not already exists
mkdir -p ./build

if [ "$CMD" == "local" ]; then
	# Get current version from git tags
	version=$(git describe --tags)

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
	docker build -f "$Dockerfile" -t $DockerImage:$timestamp . --build-arg GOARCH=$GOARCH --build-arg GOOS=$GOOS;

fi

if [ "$CMD" == "remote" ]; then
  echo "build from remote git repo"
	DockerImage="$GITHUB_USER.$BRANCH.boringproxy-$GOOS-$GOARCH"
	Dockerfile="Dockerfile_remote"

	# Build docker image(s)
	REPO="https://github.com/$GITHUB_USER/boringproxy.git"
	docker build -f "$Dockerfile" -t $DockerImage:$timestamp . --build-arg GOARCH=$GOARCH --build-arg GOOS=$GOOS --build-arg BRANCH=$BRANCH --build-arg REPO=$REPO;
fi


# if DockerImage is set, continue
if [ -n "$DockerImage" ]; then {
	# Prune intermediate images
	docker image prune -f --filter label=boringproxy=builder

	# Copy exec from image
	docker cp $(docker create $DockerImage:$timestamp):/boringproxy ./build/$DockerImage;

	# Remove temp container
	docker rm $(docker container ls -n 1  | awk '{ print $1 }' | grep -v CONTAINER)

	# Remove image
	docker rmi $DockerImage:$timestamp;
}
fi