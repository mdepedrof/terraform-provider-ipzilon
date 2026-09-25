# Reserve the first available /24 in a scope
resource "ipzilon_next_network" "example" {
  scope_id      = ipzilon_scope.example.id
  prefix_length = 24
  name          = "landing-zone-network"
  description   = "Landing Zone Network"
}

# The assigned CIDR is available after apply:
# ipzilon_next_network.example.cidr → "10.0.1.0/24"
