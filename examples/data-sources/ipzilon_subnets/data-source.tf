# List all subnets in a network
data "ipzilon_subnets" "all" {
  network_id = 1
}

output "subnet_cidrs" {
  value = [for s in data.ipzilon_subnets.all.items : s.cidr]
}

# Subnets inside a zone (IPzilon >= 3.1.0); network_id is not required
data "ipzilon_subnets" "pooled" {
  zone_id = data.ipzilon_network_zones.pooled.items[0].id
}

# Subnets of a network outside every zone (IPzilon >= 3.1.0)
data "ipzilon_subnets" "outside_zones" {
  network_id = ipzilon_network.example.id
  no_zone    = true
}
