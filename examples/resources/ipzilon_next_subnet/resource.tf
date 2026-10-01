# Reserve the first available /27 in a network
resource "ipzilon_next_subnet" "example" {
  network_id    = ipzilon_network.example.id
  prefix_length = 27
  name          = "web-tier"
  description   = "Web Tier Subnet"
}

# The assigned CIDR is available after apply:
# ipzilon_next_subnet.example.cidr → "10.0.1.0/27"

# Reserve the first available /28 inside a network zone (IPzilon >= 3.1.0).
# Without zone_id, in a network that has zones the block is taken outside
# every zone. zone_id never forces a replacement.
resource "ipzilon_next_subnet" "pooled" {
  network_id    = ipzilon_network.example.id
  zone_id       = ipzilon_network_zone.example.id
  prefix_length = 28
  name          = "hp-pooled-01"
}
