# Root landing zone
resource "ipzilon_landing_zone" "root" {
  hub_id      = ipzilon_hub.example.id
  name        = "shared-services"
  description = "Shared Services Landing Zone"
}

# Child landing zone
resource "ipzilon_landing_zone" "child" {
  hub_id    = ipzilon_hub.example.id
  parent_id = ipzilon_landing_zone.root.id
  name      = "app-tier"
}
