#!/bin/bash
set -euo pipefail

SCRIPT_DIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )
PROJECT_ROOT=$( cd "$( dirname "${SCRIPT_DIR[0]}" )" && pwd )

# Clear out the old builds/zips etc
rm -rf "${PROJECT_ROOT:?}/bin"

# Build the windows binary for the CA
GOOS=windows GOARCH=386 go build -o "${PROJECT_ROOT}/bin/sshizzle-ca.exe" "${PROJECT_ROOT}/cmd/sshizzle-ca/sshizzle-ca.go"

# Build sshizzle-host and sshizzle-agent binaries
go build -o "${PROJECT_ROOT}/bin/sshizzle-agent" "${PROJECT_ROOT}/cmd/sshizzle-agent/sshizzle-agent.go"
go build -o "${PROJECT_ROOT}/bin/sshizzle-host" "${PROJECT_ROOT}/cmd/sshizzle-host/sshizzle-host.go"
go build -o "${PROJECT_ROOT}/bin/sshizzle-convert" "${PROJECT_ROOT}/cmd/sshizzle-convert/sshizzle-convert.go"
