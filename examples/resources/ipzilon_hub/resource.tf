resource "ipzilon_hub" "example" {
  # Changing site_id moves the hub to another site in place, keeping its id and
  # the ids of its scopes, networks and subnets (IPzilon >= 3.2.0). IPzilon
  # rejects a site of another type, a repeated hub name or an overlapping
  # address_space in the destination site.
  site_id       = 1
  name          = "hub-prod"
  address_space = "10.0.0.0/16"
  location      = "West Europe"
  description   = "Production Hub"
}
