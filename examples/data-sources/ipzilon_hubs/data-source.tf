# List all hubs in a site
data "ipzilon_hubs" "all" {
  site_id = 1
}

# Lookup a single hub by ID
data "ipzilon_hubs" "single" {
  id = 1
}

# List hubs in a site filtered by address_space or name
data "ipzilon_hubs" "filtered" {
  site_id       = 1
  address_space = "10.0.0.0/16"
}

output "hub_cidr" {
  value = data.ipzilon_hubs.all.items[0].address_space
}
