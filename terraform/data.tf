// Get our currenly auth'd creds
data "azurerm_client_config" "current" {}

// Get our public IP
data "http" "client_ip" {
  url = "https://ifconfig.co"
}

data "azuread_user" "current" {
  object_id = data.azurerm_client_config.current.object_id
}