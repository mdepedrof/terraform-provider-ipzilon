# Requires IPzilon >= 3.1.0.
# Reserve the first empty /25 of a network as a zone. The block overlaps no
# zone and no subnet of the network.
resource "ipzilon_next_network_zone" "example" {
  network_id    = ipzilon_network.example.id
  prefix_length = 25
  name          = "personal_zone"
  description   = "Subnets of personal host pools"
}

# The assigned CIDR is available after apply:
# ipzilon_next_network_zone.example.cidr → "10.0.18.0/25"
