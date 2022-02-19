#!/bin/bash

export COMPOSE_PROJECT_NAME="bpc-nginx"
docker-compose down; # Stop containers if running
docker-compose up -d;
docker-compose logs -f;