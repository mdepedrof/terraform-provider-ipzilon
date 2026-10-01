# Reserve the last available /27 in a network (useful for management subnets)
resource "ipzilon_last_subnet" "mgmt" {
  network_id    = ipzilon_network.example.id
  prefix_length = 27
  name          = "mgmt-tier"
  description   = "Management Subnet"
}

# The assigned CIDR is available after apply:
# ipzilon_last_subnet.mgmt.cidr → "10.0.1.224/27"

# Reserve the last available /28 inside a network zone (IPzilon >= 3.1.0).
resource "ipzilon_last_subnet" "pooled_tail" {
  network_id    = ipzilon_network.example.id
  zone_id       = ipzilon_network_zone.example.id
  prefix_length = 28
  name          = "hp-pooled-tail"
}
