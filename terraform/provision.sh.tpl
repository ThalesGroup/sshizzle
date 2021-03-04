#!/bin/bash
set -euo pipefail
# USE FOR DEMOS ONLY.

# This script sets up the SSH daemon on the server to accept user certificates
# that have been signed by the configured CA.

# In real life, this config should be baked into the gold images
# that are used for deploying Linux servers

echo -n ${ca_certificate} | base64 -d > /etc/ssh/user_ca.pub
if [ -z $(grep "TrustedUserCAKeys /etc/ssh/user_ca.pub" "/etc/ssh/sshd_config") ]; then
  echo "TrustedUserCAKeys /etc/ssh/user_ca.pub" >> /etc/ssh/sshd_config
fi
systemctl restart sshd
