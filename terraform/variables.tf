variable "login_email" {
  type        = string
  description = "Email address you use to login into Azure. E.g. joe.bloggs@somecorp.io"
}

variable "location" {
  type        = string
  description = "Azure Region to deploy to (e.g. uksouth)"
}

variable "prefix" {
  type         = string
  description  = "Azure name prefix"
}

variable "ca_key_size" {
  type        = number
  description = "Size in bits of the RSA CA key (2048, 3072, or 4096)"
  default     = 4096
}
