## Table of Contents

- [Introduction](#introduction)
- [Components](#components)
  - [sshizzle-agent](#sshizzle-agent)
  - [sshizzle-ca](#sshizzle-ca)
  - [sshizzle-host](#sshizzle-host)
- [Getting Started](#getting-started)

## Introduction

Full introduction available [on LinkedIn](https://www.linkedin.com/pulse/zero-trust-ssh-microsoft-azure-jon-seager)!

SSHizzle is the combination of a serverless SSH Certificate Authority and a replacement SSH agent designed to simplify the management of SSH users and authentication both for administrators and users on Microsoft Azure. SSHizzle was inspired by the work of Jeremy Stott ([@stoggi](https://github.com/stoggi)) and the [sshrimp](https://github.com/stoggi/sshrimp) project which implements a similar solution for AWS.

SSHizzle strives to be as simple to configure, use and run as possible. It benefits from tight integration with Microsoft Azure Active Directory, Azure Key Vault and works seamlessly with almost all existing workflows. SShizzle simplifies the adoption and management of SSH Certificates.

In summary, SSHizzle aims to:

- Reduce the complexity of on-boarding new users into environments
- Provide a good user experience to engineers and developers
- Increase the observability of SSH credentials
- Eliminate the need to distribute many public keys in production
- Reduce the need to manage the lifecycle of SSH keys in production
- Be compatible with existing SSH-based workflows
- Require minimal configuration of hosts and servers

## Components

There are 3 provided commands as part of this package:

- **sshizzle-agent**: An SSH Agent implementation that can authenticate with Azure Active Directory and invoke the SSH Certificate Authority serverless function
- **sshizzle-ca**: A lightweight, serverless SSH Certificate Authority that signs SSH public keys with an RSA key stored in an Azure Key Vault.
- **sshizzle-host** - A debugging/testing tool to configure an SSH server to trust the public key of the CA key in the Key Vault

The [terraform](./terraform) directory contains [Terraform](https://terraform.io) code that will deploy:

- An Azure Resource Group
- An Azure Key Vault
- An Azure Function
- An Azure Storage Account
- A Virtual Machine (a demo SSH server configured to trust the key in the Key Vault)

The Terraform code does not necessarily represent best practice and includes a number of elements that are only to facilitate testing/debugging, such as permissions on the Azure Key Vault. The code is commented heavily such as to point this out where it occurs. It will, in most cases, restrict network access to resources and only permit the IP that was used to create them to interact.

The [util](./util) directory contains scripts used to build the project, and deploy the demo environment.

### sshizzle-agent

The `sshizzle-agent` binary is designed to run in the background on a Unix-like host. It listens by default on a Unix socket located at `/tmp/sshizzle.sock`. In order to start, it requires a `.env` file present in the working directory like so:

```dotenv
AZ_TENANT_ID="34d343a-21ed-4bcd-a226-92e43245a0c5"
AZ_CLIENT_ID="231230f3c-1cd6-4aac-89ee-4d2d500b3412"
AZ_FUNC_HOST="func-sshizzle-43ds2.azurewebsites.net"
```

During provisioning with the [setup script](./util/setup-demo.sh), there will be two client IDs created. The Client ID here refers to `app-sshizzle-agent`.

**This file will be automatically created if following the steps for testing below.**

### sshizzle-ca

This needs to be compiled as a Windows binary for the Azure Function. Go is not an officially support language for Azure Functions - as such we just upload an `exe` file to be executed as part of the function. During the automation, the function is deployed using a zip file with the following structure

```bash
├── build                     # top-level directory
│   ├── bin                   # this doesn't have to be a subdirectory necessarily, path to exe is in host.json
│   │   └── sshizzle-ca.exe
│   ├── host.json             # function host config
│   └── sign-agent-key        # one folder per function for an Azure function app
│       └── function.json
```

Once a zip file with the above structure has been created, the function can be deployed with the Azure CLI:

```
az functionapp deployment source config-zip -g <RESOURCE_GROUP> -n <FUNCTION_NAME> --src <PATH_TO_ZIP>
```

### sshizzle-host

A small utility that configures SSH servers to trust the CA's public key from the configured Azure Key Vault. At the moment, the values for the key vault name and key name are hardcoded to those setup using the automation provided in this repository. In production, it is unlikely this tool would be required, a more sensible approach would be to ensure the public key is present in OS base images.

If there is a Managed System Identity (MSI) present (available to Azure VMs), then that is used to authenticate, otherwise, the tool will fallback to authenticating using the `az` CLI.

To provision a machine for use with the sshizzle CA, make sure you are logged into the `az` CLI tool (or have an MSI available), and run:

```
sudo -E ./bin/sshizzle-host
```

Superuser rights are required as the tool will edit the SSH daemon config at `/etc/ssh/sshd_config` on the machine, and restart the SSH daemon. Details will be in the logs at stdout.

## Getting Started

Before attempting to run the deployment automation, please ensure the following tools are in your PATH:

- `terraform`
- `az`
- `jq`
- `zip`
- `go`

Also ensure that you are logged into the Azure CLI with `az account show`. If not, then login with `az login`.

```bash
$ mkdir -p "${GOPATH}/src/github.com/thalesgroup"
$ git clone git@github.com:thalesgroup/sshizzle.git "${GOPATH}/src/github.com/thalesgroup/sshizzle"
$ cd "${GOPATH}/src/github.com/thalesgroup/sshizzle"
$ ./util/setup-demo.sh
```

All of the basics should now be in place. To test the deployment, add the lines to your `~/.ssh/config` file indicated by the `setup-demo.sh` script. Before trying to authenticate with the machine, make sure the agent is running with `./bin/sshizzle-agent`.

```
$ ssh sshizzle-vm
```

This _should_ pop a browser window for authentication, then log you into the VM. It may take a few seconds the first time, the Azure Function invocation sometimes takes a few seconds to spin up.
