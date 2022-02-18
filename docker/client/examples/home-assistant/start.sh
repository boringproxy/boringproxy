#!/bin/bash

export COMPOSE_PROJECT_NAME="bpc-homeassistant"
docker-compose down; # Stop containers if running
docker-compose up -d;
docker-compose logs -f;