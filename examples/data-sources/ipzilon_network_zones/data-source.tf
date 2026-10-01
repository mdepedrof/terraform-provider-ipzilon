# Requires IPzilon >= 3.1.0.

# Look a zone up by its CIDR only, without any id
data "ipzilon_network_zones" "pooled" {
  cidr = "10.0.16.0/23"
}

# A zone name is only unique within its network: add network_id to narrow it
data "ipzilon_network_zones" "personal" {
  network_id = ipzilon_network.example.id
  name       = "personal_zone"
}

output "pooled_zone_id" {
  value = data.ipzilon_network_zones.pooled.items[0].id
}
