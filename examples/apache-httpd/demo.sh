#!/bin/bash

# Check if we have swarm mode
if docker node ls > /dev/null 2>&1; then
  echo "Swarm mode enabled"
else
  echo "Swarm mode not enabled. Exiting!"; exit
fi

# Check if there is a certificate stored in the certs folder
if [ "$(ls -A certs)" ]; then
  echo "Certificate found."
else
  echo "No certficate in certs folder found? ca.pem? client.pem? key.pem? Exiting..."
  exit
fi

# Copy the bosnd binary to the build directory
cp ../../bosnd .

# Build the Apache httpd container including the bosnd binary
# For more informaiton see Dockerfile
docker build . -t bosnd-httpd:1.0

# Remove the bosnd binary after build
rm bosnd

# After the build create the overlay network
docker network create --attachable --driver=overlay bosnd_net

# Start the stack
docker stack deploy -c docker-stack.yml bosnd

# Wait for user input
sleep 2
echo -n "Docker stack started! Point your browser to http://127.0.0.1:3456! Press [ENTER] to stop the stack: "
read

# Remove the stack
docker stack rm bosnd