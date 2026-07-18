resource "ipzilon_hub" "example" {
  site_id       = 1
  name          = "hub-prod"
  address_space = "10.0.0.0/16"
  location      = "West Europe"
  description   = "Production Hub"
}
