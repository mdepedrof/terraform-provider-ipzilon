# List all hubs in a site
data "ipzilon_hubs" "all" {
  site_id = 1
}

# Lookup a single hub by ID
data "ipzilon_hubs" "single" {
  id = 1
}

output "hub_cidr" {
  value = data.ipzilon_hubs.all.items[0].address_space
}
