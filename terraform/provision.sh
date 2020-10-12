#!/bin/bash
set -euo pipefail
# USE FOR DEMOS ONLY.

# This script simple gets the public key of the CA, and sets up the SSH
# daemon on the server to accept user certificates that have been
# signed by the CA

# In real life, this config should be baked into the gold images
# that are used for deploying Linux servers

# Get the latest version number for sshizzle from Github
REPO="ThalesGroup/sshizzle"
VERSION=$(wget -qO- "https://github.com/${REPO}/releases" | grep -Po "/${REPO}/releases/tag/\K[0-9]+\.[0-9]+\.[0-9]+")

# Download the latest sshizzle-host binary
mkdir -p /usr/local/bin
wget -qO /usr/local/bin/sshizzle-host "https://github.com/${REPO}/releases/download/${VERSION}/sshizzle-host-${VERSION}-linux-amd64"
chmod 755 /usr/local/bin/sshizzle-host

# Wait for networking rules and access policies to propogate
sleep 15 
/usr/local/bin/sshizzle-host

