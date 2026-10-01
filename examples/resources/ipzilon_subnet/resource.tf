resource "ipzilon_subnet" "example" {
  network_id  = ipzilon_network.example.id
  name        = "web-tier"
  cidr        = "10.0.1.0/27"
  description = "Web Tier Subnet"
}

# Declare that a subnet with an explicit CIDR belongs to a zone (IPzilon >= 3.1.0).
# IPzilon checks on create and update that the CIDR is inside the zone.
resource "ipzilon_subnet" "pooled" {
  network_id = ipzilon_network.example.id
  zone_id    = ipzilon_network_zone.example.id
  name       = "hp-pooled-02"
  cidr       = "10.0.16.16/28"
}
