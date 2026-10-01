# Requires IPzilon >= 3.1.0.
# A zone groups the subnets of a network whose CIDR falls inside it.
resource "ipzilon_network_zone" "example" {
  network_id  = ipzilon_network.example.id
  name        = "pooled_zone_1"
  cidr        = "10.0.16.0/23"
  description = "Subnets of pooled host pools"
}
