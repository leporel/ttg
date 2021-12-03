#!/bin/bash

if ! openssl version>/dev/null; then
      echo "openssl not found in this system"
      exit
fi

openssl version
echo -n "Enter your host (eg. 127.0.0.1 or my-host.no-ip.org): > "
read -r host
openssl req -newkey rsa:2048 -sha256 -nodes -keyout key.pem -x509 -days 4365 -out cert.pem -subj "/CN=$host"