// Create a new resource group for the test server
resource "azurerm_resource_group" "rg-sshizzle-test-server" {
  name     = "rg-sshizzle-test-server"
  location = var.location
}

// Create a virtual network the VM
resource "azurerm_virtual_network" "vnet-sshizzle" {
  name                = "vnet-sshizzle"
  address_space       = ["10.200.200.0/24"]
  location            = azurerm_resource_group.rg-sshizzle-test-server.location
  resource_group_name = azurerm_resource_group.rg-sshizzle-test-server.name
}

// Create a subnet for the VM
resource "azurerm_subnet" "snet-sshizzle" {
  name                 = "snet-sshizzle"
  resource_group_name  = azurerm_resource_group.rg-sshizzle-test-server.name
  virtual_network_name = azurerm_virtual_network.vnet-sshizzle.name
  address_prefixes     = ["10.200.200.0/24"]
  service_endpoints    = ["Microsoft.KeyVault"]

  // This is a hack because you cannot current add Key Vault network_acl seperately from the Key Vault definition:
  // https://github.com/terraform-providers/terraform-provider-azurerm/issues/3130
  provisioner "local-exec" {
    command = "az keyvault network-rule add --name ${azurerm_key_vault.kv-sshizzle.name} --subnet ${azurerm_subnet.snet-sshizzle.id}"
  }
  depends_on = [azurerm_key_vault.kv-sshizzle]
}

// Create a public IP for the VM
resource "azurerm_public_ip" "pip-sshizzle-test-server" {
  name                = "pip-sshizzle-test-server"
  location            = azurerm_resource_group.rg-sshizzle-test-server.location
  resource_group_name = azurerm_resource_group.rg-sshizzle-test-server.name
  allocation_method   = "Static"
}

// Create the  VM NIC and associate with the Public IP
resource "azurerm_network_interface" "nic-sshizzle-test-server" {
  name                = "nic-sshizzle-test-server"
  location            = azurerm_resource_group.rg-sshizzle-test-server.location
  resource_group_name = azurerm_resource_group.rg-sshizzle-test-server.name

  ip_configuration {
    name                          = "default"
    subnet_id                     = azurerm_subnet.snet-sshizzle.id
    private_ip_address_allocation = "Dynamic"
    public_ip_address_id          = azurerm_public_ip.pip-sshizzle-test-server.id
  }
}

// Setup an NSG and associate with the NIC
resource "azurerm_network_security_group" "nsg-sshizzle" {
  name                = "nsg-sshizzle"
  location            = azurerm_resource_group.rg-sshizzle-test-server.location
  resource_group_name = azurerm_resource_group.rg-sshizzle-test-server.name

  // Create a rule that allows SSH traffic *only* from the machine deploying using Terraform
  security_rule {
    name                       = "SSH"
    priority                   = 1001
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "22"
    source_address_prefix      = chomp(data.http.client_ip.body)
    destination_address_prefix = "*"
  }
}

// Associate the NSG with the VM's Network Interface
resource "azurerm_network_interface_security_group_association" "nsga-sshizzle" {
  network_interface_id      = azurerm_network_interface.nic-sshizzle-test-server.id
  network_security_group_id = azurerm_network_security_group.nsg-sshizzle.id
}

// Generate a random password for the VM
resource "random_password" "vm-password" {
  length           = 20
  lower            = true
  min_lower        = 1
  upper            = true
  min_upper        = 1
  number           = true
  min_numeric      = 1
  special          = true
  min_special      = 1
  override_special = "_%@"
}

// Retrieve ssh CertificateAuthority from Azure Key Vault
data "external" "ssh_ca" {
  program = ["../bin/sshizzle-convert"]
  query = {
    E = azurerm_key_vault_key.key-sshizzle.e
    N = azurerm_key_vault_key.key-sshizzle.n
  }
}

// Render provision script with embedded CertificateAuthority
data "template_file" "provision" {
  template = file("${path.module}/provision.sh.tpl")
  vars = {
    ca_certificate = base64encode(data.external.ssh_ca.result.ca)
  }
}

// Create the VM
resource "azurerm_virtual_machine" "vm-sshizzle-test-server" {
  name                  = "vm-sshizzle-test-server"
  location              = azurerm_resource_group.rg-sshizzle-test-server.location
  resource_group_name   = azurerm_resource_group.rg-sshizzle-test-server.name
  network_interface_ids = [azurerm_network_interface.nic-sshizzle-test-server.id]
  vm_size               = "Standard_B1s"

  delete_os_disk_on_termination    = true
  delete_data_disks_on_termination = true

  storage_image_reference {
    publisher = "Canonical"
    offer     = "UbuntuServer"
    sku       = "18.04-LTS"
    version   = "latest"
  }
  storage_os_disk {
    name              = "disk-sshizzle-test-server"
    caching           = "ReadWrite"
    create_option     = "FromImage"
    managed_disk_type = "Standard_LRS"
  }
  os_profile {
    computer_name  = "sshizzle-test-server"
    admin_username = split("@", var.login_email)[0]
    admin_password = random_password.vm-password.result
    custom_data    = data.template_file.provision.rendered
  }
  os_profile_linux_config {
    disable_password_authentication = false
  }
}



