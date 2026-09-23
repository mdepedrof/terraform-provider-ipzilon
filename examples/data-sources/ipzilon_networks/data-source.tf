# List all networks in a scope
data "ipzilon_networks" "all" {
  scope_id = 1
}

output "network_cidrs" {
  value = [for n in data.ipzilon_networks.all.items : n.cidr]
}
