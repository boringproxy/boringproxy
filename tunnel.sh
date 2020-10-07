#!/bin/bash

server=$1
token=$2
domain=$3
localPort=$4

api="https://$server/api"

echo "Creating tunnel"

json=$(curl -s -H "Authorization: bearer $token" -X POST "$api/tunnels?domain=$domain")

serverAddress=$(echo "$json" | jq -r '.server_address')
serverPort=$(echo "$json" | jq -r '.server_port')
username=$(echo "$json" | jq -r '.username')
tunnelPort=$(echo "$json" | jq -r '.tunnel_port')
tunnelPrivateKey=$(echo "$json" | jq -r '.tunnel_private_key')

# TODO: It would be nice if we could avoid writing the private key to disk.
# I tried process substition but it didn't work.
keyFile=$(mktemp)
chmod 0600 $keyFile
printf -- "$tunnelPrivateKey" > $keyFile

echo "Connecting to tunnel"

ssh -i $keyFile \
    -NR 127.0.0.1:$tunnelPort:127.0.0.1:$localPort \
    $username@$serverAddress -p $serverPort

echo "Cleaning up"

rm $keyFile
curl -s -H "Authorization: bearer $token" -X DELETE "$api/tunnels?domain=$domain"
