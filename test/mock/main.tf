# Module exercised by `make testmock`: terraform test with mock_provider must be
# able to set items on every list data source (issue #26). Never run against a
# live API.

terraform {
  required_providers {
    ipzilon = {
      source = "mdepedrof/ipzilon"
    }
  }
}

provider "ipzilon" {}

# Reproduction from issue #26.
data "ipzilon_hubs" "regional" {
  name = "x"

  lifecycle {
    postcondition {
      condition     = length(self.items) == 1
      error_message = "len"
    }
  }
}

output "hub" {
  value = data.ipzilon_hubs.regional.items[0].id
}

data "ipzilon_ip_addresses" "gw" {
  subnet_id = 1
  address   = "10.0.1.17"
}

data "ipzilon_network_zones" "pooled" {
  cidr = "10.0.16.0/23"
}

data "ipzilon_networks" "avd" {
  cidr = "10.0.16.0/22"
}

data "ipzilon_scopes" "avd" {
  kind = "project"
  name = "avd"
}

data "ipzilon_sites" "all" {}

data "ipzilon_subnets" "all" {
  network_id = 1
}

output "ip_addresses_count" {
  value = length(data.ipzilon_ip_addresses.gw.items)
}

output "ip_addresses_ids" {
  value = [for i in data.ipzilon_ip_addresses.gw.items : i.id]
}

output "network_zones_count" {
  value = length(data.ipzilon_network_zones.pooled.items)
}

output "network_zones_ids" {
  value = [for i in data.ipzilon_network_zones.pooled.items : i.id]
}

output "networks_count" {
  value = length(data.ipzilon_networks.avd.items)
}

output "networks_ids" {
  value = [for i in data.ipzilon_networks.avd.items : i.id]
}

output "networks_first_name" {
  value = try(data.ipzilon_networks.avd.items[0].name, null)
}

output "scopes_count" {
  value = length(data.ipzilon_scopes.avd.items)
}

output "scopes_ids" {
  value = [for i in data.ipzilon_scopes.avd.items : i.id]
}

output "sites_count" {
  value = length(data.ipzilon_sites.all.items)
}

output "sites_ids" {
  value = [for i in data.ipzilon_sites.all.items : i.id]
}

output "subnets_count" {
  value = length(data.ipzilon_subnets.all.items)
}

output "subnets_ids" {
  value = [for i in data.ipzilon_subnets.all.items : i.id]
}
