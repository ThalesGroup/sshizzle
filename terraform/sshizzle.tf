locals {
  // Generate name in advance to avoid cyclic dependency
  keyvault_name = "${var.prefix}-kv-sshizzle"
}

// Create a resource group
resource "azurerm_resource_group" "rg-sshizzle" {
  name     = "rg-sshizzle"
  location = var.location
}

// Create the Key Vault
resource "azurerm_key_vault" "kv-sshizzle" {
  name                     = local.keyvault_name
  location                 = azurerm_resource_group.rg-sshizzle.location
  resource_group_name      = azurerm_resource_group.rg-sshizzle.name
  tenant_id                = data.azurerm_client_config.current.tenant_id
  sku_name                 = "standard"
  soft_delete_enabled      = false
  purge_protection_enabled = false

  // Allow network access to possible Azure Functions IP space and Terraform client IP
  network_acls {
    default_action = "Deny"
    bypass         = "AzureServices"
    ip_rules = concat(
      [chomp(data.http.client_ip.body)],
      split(",", azurerm_function_app.func-sshizzle.possible_outbound_ip_addresses),
    )
  }

  // Allow the Function App MSI access to fetch and sign with keys
  access_policy {
    tenant_id       = data.azurerm_client_config.current.tenant_id
    object_id       = azurerm_function_app.func-sshizzle.identity[0].principal_id
    key_permissions = ["get", "sign"]
  }

  // Catch all permissions for the current user (assuming Azure CLI auth, this should be you!)
  // Used for testing - probably shouldn't do this in production
  access_policy {
    tenant_id       = data.azurerm_client_config.current.tenant_id
    object_id       = data.azurerm_client_config.current.object_id
    key_permissions = ["get", "list", "update", "create", "import", "delete", "recover", "backup", "restore"]
  }
}

// Create the CA key - an Azure generated 2048 bit RSA key
resource "azurerm_key_vault_key" "key-sshizzle" {
  name         = "sshizzle"
  key_vault_id = azurerm_key_vault.kv-sshizzle.id
  key_type     = "RSA"
  key_size     = 2048
  key_opts     = ["sign", "verify"]
}

// Create a storage account to back the App Service Plan
resource "azurerm_storage_account" "sasshizzle" {
  name                     = "${var.prefix}sasshizzle"
  resource_group_name      = azurerm_resource_group.rg-sshizzle.name
  location                 = azurerm_resource_group.rg-sshizzle.location
  account_tier             = "Standard"
  account_replication_type = "LRS"
}

// Create an App Service plan to enable billing for the Azure Function
resource "azurerm_app_service_plan" "plan-sshizzle" {
  name                = "plan-sshizzle"
  location            = azurerm_resource_group.rg-sshizzle.location
  resource_group_name = azurerm_resource_group.rg-sshizzle.name
  kind                = "FunctionApp"

  sku {
    tier = "Dynamic"
    size = "Y1"
  }
}

// Generate a random identifier for the Azure Function to avoid collisions
resource "random_id" "function-id" {
  byte_length = 4
}

// Create an app registration for the agent
resource "azuread_application" "app-sshizzle-agent" {
  name                       = "app-sshizzle-agent"
  available_to_other_tenants = false
  type                       = "native"
  owners                     = [data.azurerm_client_config.current.object_id]
  reply_urls                 = ["http://localhost:8080/callback"]
  // Give application access to the Graph API and allow users to Sign In
  required_resource_access {
    resource_app_id = "00000003-0000-0000-c000-000000000000"
    resource_access {
      id   = "e1fe6dd8-ba31-4d61-89e7-88639da4683d"
      type = "Scope"
    }
  }
}

// Create an app registration for the CA
resource "azuread_application" "app-sshizzle-ca" {
  name                       = "app-sshizzle-ca"
  owners                     = [data.azurerm_client_config.current.object_id]
  homepage                   = "https://func-sshizzle-${lower(random_id.function-id.b64_url)}.azurewebsites.net"
  identifier_uris            = ["https://func-sshizzle-${lower(random_id.function-id.b64_url)}.azurewebsites.net"]
  reply_urls                 = ["https://func-sshizzle-${lower(random_id.function-id.b64_url)}.azurewebsites.net/.auth/login/aad/callback"]
  type                       = "webapp/api"
  available_to_other_tenants = false

  required_resource_access {
    resource_app_id = "00000002-0000-0000-c000-000000000000"
    resource_access {
      id   = "311a71cc-e848-46a1-bdf8-97ff7156d8e6"
      type = "Scope"
    }
  }
}

// Generate a random password for the CA service principal
resource "random_password" "ca-password" {
  length           = 32
  special          = true
  override_special = "_%@"
}

// Create the application password
resource "azuread_application_password" "apppw-sshizzle-ca" {
  application_object_id = azuread_application.app-sshizzle-ca.id
  value                 = random_password.ca-password.result
  end_date_relative     = "8760h"
}

// Create a service principal for the CA
resource "azuread_service_principal" "sp-sshizzle-ca" {
  application_id               = azuread_application.app-sshizzle-ca.application_id
  app_role_assignment_required = false
  // This Tag is needed to make it an "Enterprise Application"
  tags = ["WindowsAzureActiveDirectoryIntegratedApp"]
}

// Create the Azure Function
resource "azurerm_function_app" "func-sshizzle" {
  name                       = "func-sshizzle-${lower(random_id.function-id.b64_url)}"
  location                   = azurerm_resource_group.rg-sshizzle.location
  resource_group_name        = azurerm_resource_group.rg-sshizzle.name
  app_service_plan_id        = azurerm_app_service_plan.plan-sshizzle.id
  storage_account_name       = azurerm_storage_account.sasshizzle.name
  storage_account_access_key = azurerm_storage_account.sasshizzle.primary_access_key

  version                 = "~3"
  https_only              = true
  client_affinity_enabled = true
  enable_builtin_logging  = true

  // Force users to sign in with Azure AD before they can invoke the function
  auth_settings {
    enabled                       = true
    default_provider              = "AzureActiveDirectory"
    issuer                        = "https://sts.windows.net/${data.azurerm_client_config.current.tenant_id}/"
    token_store_enabled           = true
    unauthenticated_client_action = "RedirectToLoginPage"

    active_directory {
      client_id         = azuread_application.app-sshizzle-ca.application_id
      client_secret     = azuread_application_password.apppw-sshizzle-ca.id
      allowed_audiences = ["https://func-sshizzle-${lower(random_id.function-id.b64_url)}.azurewebsites.net"]
    }
  }

  app_settings = {
    KV_NAME = local.keyvault_name
  }

  identity {
    type = "SystemAssigned"
  }
}

