# Manage a specific IP address within a subnet.
# The IP record must already exist (auto-created when the subnet was created).
resource "ipzilon_ip_address" "gateway" {
  subnet_id   = ipzilon_next_subnet.example.id
  address     = "10.0.1.1"
  status      = "reserved"
  hostname    = "gw-web-tier"
  description = "Web Tier Gateway"
}

# Available attributes after apply:
# ipzilon_ip_address.gateway.address          → "10.0.1.1"
# ipzilon_ip_address.gateway.hostname         → "gw-web-tier"
# ipzilon_ip_address.gateway.status           → "reserved"
# ipzilon_ip_address.gateway.is_azure_reserved → true  (Azure reserves .1)
