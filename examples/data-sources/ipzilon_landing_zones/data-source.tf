# List all root landing zones in a hub
data "ipzilon_landing_zones" "roots" {
  hub_id    = 1
  root_only = true
}

# List child landing zones of a specific parent
data "ipzilon_landing_zones" "children" {
  hub_id    = 1
  parent_id = data.ipzilon_landing_zones.roots.items[0].id
}
