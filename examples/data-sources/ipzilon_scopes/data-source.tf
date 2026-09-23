# List all root scopes in a hub
data "ipzilon_scopes" "roots" {
  hub_id    = 1
  root_only = true
}

# List child scopes of a specific parent
data "ipzilon_scopes" "children" {
  hub_id    = 1
  parent_id = data.ipzilon_scopes.roots.items[0].id
}

# List only "project" scopes in a hub
data "ipzilon_scopes" "projects" {
  hub_id = 1
  kind   = "project"
}
