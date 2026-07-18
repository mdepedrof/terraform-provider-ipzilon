resource "ipzilon_network" "example" {
  landing_zone_id = ipzilon_landing_zone.child.id
  name            = "spoke-app"
  cidr            = "10.0.1.0/24"
  description     = "Application Spoke Network"
}
