
output "tenant-id" {
  value = data.azurerm_client_config.current.tenant_id
}

output "app-sshizzle-agent" {
  value = azuread_application.app-sshizzle-agent.application_id
}

output "app-sshizzle-ca" {
  value = azuread_application.app-sshizzle-ca.application_id
}

output "function-hostname" {
  value = azurerm_function_app.func-sshizzle.default_hostname
}

output "test-server-ip" {
  value = azurerm_public_ip.pip-sshizzle-test-server.ip_address
}

output "admin-user" {
  value = split("@", var.login_email)[0]
}

output "vm-user-password" {
  value = random_password.vm-password.result
  sensitive = true
}

