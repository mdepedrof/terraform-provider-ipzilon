# Requires IPzilon >= 3.1.0.
# Reserve the last empty /25 of a network as a zone.
resource "ipzilon_last_network_zone" "example" {
  network_id    = ipzilon_network.example.id
  prefix_length = 25
  name          = "personal_zone"
}

# The assigned CIDR is available after apply:
# ipzilon_last_network_zone.example.cidr → "10.0.19.128/25"
