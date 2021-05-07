#!/bin/bash
set -euo pipefail

SCRIPT_DIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )
PROJECT_ROOT=$( cd "$( dirname "${SCRIPT_DIR[0]}" )" && pwd )

if ! command -v jq >/dev/null ||
   ! command -v az >/dev/null ||
   ! command -v terraform >/dev/null ||
   ! command -v zip >/dev/null ||
   ! command -v go >/dev/null; then
  echo "You must have jq, zip, terraform, az and go installed and in your PATH" >&2
  exit 1
fi

if [[ ! -f "${PROJECT_ROOT}/terraform/terraform.tfvars" ]]; then
  read -r -p "Enter project location: " LOCATION
  echo "location = \"${LOCATION}\"" > "${PROJECT_ROOT}/terraform/terraform.tfvars"
  read -r -p "Enter the email address you use to login into Azure. (e.g joe.bloggs@gmail.com): " EMAIL
  echo "login_email = \"${EMAIL}\"" >> "${PROJECT_ROOT}/terraform/terraform.tfvars"
  read -r -p "Enter project prefix: " PREFIX
  echo "prefix = \"${PREFIX}\"" >> "${PROJECT_ROOT}/terraform/terraform.tfvars"
fi

# Build all the things
"${SCRIPT_DIR}/build-all.sh"

# Deploy Azure resources
cd "${PROJECT_ROOT}/terraform" || exit 1
terraform init
terraform apply -auto-approve

# Create deployment folder
mkdir -p "${SCRIPT_DIR}/build/bin"
cp "${PROJECT_ROOT}/host.json" "${SCRIPT_DIR}/build/host.json"
cp "${PROJECT_ROOT}/bin/sshizzle-ca.exe" "${SCRIPT_DIR}/build/bin/sshizzle-ca.exe"
cp -r "${PROJECT_ROOT}/sign-agent-key" "${SCRIPT_DIR}/build/sign-agent-key"

# Zip up the deploy folder
cd "${SCRIPT_DIR}/build" || exit 1
zip -r "${PROJECT_ROOT}/bin/func-sshizzle.zip" ./*
cd "${PROJECT_ROOT}" || exit 1
rm -rf "${SCRIPT_DIR}/build"

# Get the variables for this deployment
cd "${PROJECT_ROOT}/terraform"
AZ_TENANT_ID=$(terraform output -json | jq -r '."tenant-id".value')
AZ_CLIENT_ID=$(terraform output -json | jq -r '."app-sshizzle-agent".value')
AZ_FUNC_HOST=$(terraform output -json | jq -r '."function-hostname".value')
SERVER_IP=$(terraform output -json | jq -r '."test-server-ip".value')
ADMIN_USER=$(terraform output -json | jq -r '."admin-user".value')
cd "${PROJECT_ROOT}" || exit 1

# Get the function name from its URL
FUNC_NAME=$(echo "${AZ_FUNC_HOST}" | cut -d"." -f1)

# Deploy the function
az functionapp deployment source config-zip -g rg-sshizzle -n "${FUNC_NAME}" --src "${PROJECT_ROOT}/bin/func-sshizzle.zip"

# Create a dotenv file for the agent
cat <<-EOF > "${PROJECT_ROOT}/.env"
AZ_TENANT_ID="${AZ_TENANT_ID}"
AZ_CLIENT_ID="${AZ_CLIENT_ID}"
AZ_FUNC_HOST="${AZ_FUNC_HOST}"
EOF

echo "Add the following to your ~/.ssh/config to access the VM:"
cat <<-EOF

Host sshizzle-vm
  Hostname ${SERVER_IP}
  User ${ADMIN_USER}
  IdentityAgent /tmp/sshizzle.sock

EOF

echo "Now run ./bin/sshizzle-agent and try 'ssh sshizzle-vm'!"
echo

