# List all sites
data "ipzilon_sites" "all" {}

# Lookup a single site by ID
data "ipzilon_sites" "single" {
  id = 1
}

# Lookup a single site by name
data "ipzilon_sites" "by_name" {
  name = "hq"
}

output "site_type" {
  value = data.ipzilon_sites.single.items[0].type
}
