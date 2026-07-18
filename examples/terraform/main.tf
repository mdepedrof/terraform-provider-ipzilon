terraform {
  required_providers {
    ipzilon = {
      source = "registry.terraform.io/ipzilon/ipzilon"
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

# Root landing zone
resource "ipzilon_landing_zone" "shared" {
  hub_id = ipzilon_hub.prod.id
  name   = "shared-services"
}

# Child landing zone
resource "ipzilon_landing_zone" "app" {
  hub_id    = ipzilon_hub.prod.id
  parent_id = ipzilon_landing_zone.shared.id
  name      = "app-tier"
}

# Network inside the child landing zone
resource "ipzilon_network" "spoke" {
  landing_zone_id = ipzilon_landing_zone.app.id
  name            = "spoke-app"
  cidr            = "10.0.1.0/24"
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

# Root landing zones for the hub
data "ipzilon_landing_zones" "roots" {
  hub_id    = ipzilon_hub.prod.id
  root_only = true
}

# Networks in the landing zone
data "ipzilon_networks" "spoke_nets" {
  landing_zone_id = ipzilon_landing_zone.app.id
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
