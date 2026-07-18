# Reserve the first available /27 in a network
resource "ipzilon_next_subnet" "example" {
  network_id    = ipzilon_network.example.id
  prefix_length = 27
  name          = "web-tier"
  description   = "Web Tier Subnet"
}

# The assigned CIDR is available after apply:
# ipzilon_next_subnet.example.cidr → "10.0.1.0/27"
