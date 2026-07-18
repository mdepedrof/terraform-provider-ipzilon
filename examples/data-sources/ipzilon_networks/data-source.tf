# List all networks in a landing zone
data "ipzilon_networks" "all" {
  landing_zone_id = 1
}

output "network_cidrs" {
  value = [for n in data.ipzilon_networks.all.items : n.cidr]
}
