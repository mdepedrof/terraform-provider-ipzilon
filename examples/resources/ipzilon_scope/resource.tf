# Root scope (must be kind = "landing_zone")
resource "ipzilon_scope" "root" {
  hub_id      = ipzilon_hub.example.id
  name        = "shared-services"
  kind        = "landing_zone"
  description = "Shared Services Landing Zone"
}

# Nested landing zone (up to 4 levels deep)
resource "ipzilon_scope" "child" {
  hub_id    = ipzilon_hub.example.id
  parent_id = ipzilon_scope.root.id
  name      = "app-tier"
  kind      = "landing_zone"
}

# A project scope nested under a landing zone.
# Once a branch is kind = "project", everything nested below it must also be "project".
resource "ipzilon_scope" "app_project" {
  hub_id    = ipzilon_hub.example.id
  parent_id = ipzilon_scope.child.id
  name      = "team-alpha"
  kind      = "project"
}
