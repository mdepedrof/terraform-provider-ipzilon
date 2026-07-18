resource "ipzilon_subnet" "example" {
  network_id  = ipzilon_network.example.id
  name        = "web-tier"
  cidr        = "10.0.1.0/27"
  description = "Web Tier Subnet"
}
