# Reserve the last available /27 in a network (useful for management subnets)
resource "ipzilon_last_subnet" "mgmt" {
  network_id    = ipzilon_network.example.id
  prefix_length = 27
  name          = "mgmt-tier"
  description   = "Management Subnet"
}

# The assigned CIDR is available after apply:
# ipzilon_last_subnet.mgmt.cidr → "10.0.1.224/27"
