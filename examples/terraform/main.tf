terraform {
  required_providers {
    ipzilon = {
      source = "registry.terraform.io/mdepedrof/ipzilon"
    }
  }
}

provider "ipzilon" {
  api_url = var.ipzilon_api_url
  token   = var.ipzilon_token
}

# Hub
resource "ipzilon_hub" "prod" {
  site_id       = var.site_id
  name          = "hub-prod"
  address_space = "10.0.0.0/16"
}

# Root scope (must be kind = "landing_zone")
resource "ipzilon_scope" "shared" {
  hub_id = ipzilon_hub.prod.id
  name   = "shared-services"
  kind   = "landing_zone"
}

# Child scope
resource "ipzilon_scope" "app" {
  hub_id    = ipzilon_hub.prod.id
  parent_id = ipzilon_scope.shared.id
  name      = "app-tier"
  kind      = "landing_zone"
}

# Network inside the child scope
resource "ipzilon_network" "spoke" {
  scope_id = ipzilon_scope.app.id
  name     = "spoke-app"
  cidr     = "10.0.1.0/24"
}

# Subnet with explicit CIDR
resource "ipzilon_subnet" "dmz" {
  network_id = ipzilon_network.spoke.id
  name       = "dmz"
  cidr       = "10.0.1.0/27"
}

# Atomically reserve the first free /27
resource "ipzilon_next_subnet" "web" {
  network_id    = ipzilon_network.spoke.id
  prefix_length = 27
  name          = "web-tier"
}

# Atomically reserve the last free /27
resource "ipzilon_last_subnet" "mgmt" {
  network_id    = ipzilon_network.spoke.id
  prefix_length = 27
  name          = "mgmt-tier"
}

# Manage a specific IP (user specifies the address)
resource "ipzilon_ip_address" "gateway" {
  subnet_id   = ipzilon_next_subnet.web.id
  address     = "10.0.1.33" # first host in the web-tier /27
  hostname    = "gw-web-tier"
  description = "Web tier gateway"
}

# Reserve the next available IP in the web subnet
resource "ipzilon_next_ip_address" "app_vm" {
  subnet_id   = ipzilon_next_subnet.web.id
  hostname    = "app-vm-01"
  description = "Application VM"
}

# --- Data sources ---

# All hubs in the site
data "ipzilon_hubs" "all" {
  site_id = var.site_id
}

# Root scopes for the hub
data "ipzilon_scopes" "roots" {
  hub_id    = ipzilon_hub.prod.id
  root_only = true
}

# Networks in the scope
data "ipzilon_networks" "spoke_nets" {
  scope_id = ipzilon_scope.app.id
}

# All subnets in the network
data "ipzilon_subnets" "all_subnets" {
  network_id = ipzilon_network.spoke.id
}

# Available IPs in the web subnet
data "ipzilon_ip_addresses" "free_ips" {
  subnet_id = ipzilon_next_subnet.web.id
  status    = "available"
}

# Outputs
output "web_subnet_cidr" {
  value = ipzilon_next_subnet.web.cidr
}

output "app_vm_address" {
  value = ipzilon_next_ip_address.app_vm.address
}

output "available_ip_count" {
  value = length(data.ipzilon_ip_addresses.free_ips.items)
}
