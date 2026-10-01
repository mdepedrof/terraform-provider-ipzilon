# List all networks in a scope
data "ipzilon_networks" "all" {
  scope_id = 1
}

output "network_cidrs" {
  value = [for n in data.ipzilon_networks.all.items : n.cidr]
}

# Lookup a network by its CIDR only, without hub_id or scope_id
# (IPzilon >= 3.2.0). The CIDR is compared as a network, so it must have no
# host bits. The same CIDR may exist in several hubs (overlapping address
# spaces): one() fails if more than one network matches.
data "ipzilon_networks" "avd" {
  cidr = "10.0.16.0/22"
}

output "avd_network_id" {
  value = one(data.ipzilon_networks.avd.items).id
}

# A lookup without matches returns items = [] (never null)
output "avd_found" {
  value = length(data.ipzilon_networks.avd.items) > 0
}
