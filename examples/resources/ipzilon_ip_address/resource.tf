# Occupy a specific free IP address within a subnet.
# status accepts "used" (default) or "reserved"; destroying the resource
# releases the address.
resource "ipzilon_ip_address" "gateway" {
  subnet_id   = ipzilon_next_subnet.example.id
  address     = "10.0.1.10"
  status      = "reserved"
  hostname    = "gw-web-tier"
  description = "Web Tier Gateway"
}

# Available attributes after apply:
# ipzilon_ip_address.gateway.address           → "10.0.1.10"
# ipzilon_ip_address.gateway.hostname          → "gw-web-tier"
# ipzilon_ip_address.gateway.status            → "reserved"
# ipzilon_ip_address.gateway.is_azure_reserved → false
