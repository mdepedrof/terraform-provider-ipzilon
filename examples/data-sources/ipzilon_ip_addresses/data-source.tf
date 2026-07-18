# List all available IPs in a subnet
data "ipzilon_ip_addresses" "available" {
  subnet_id = 1
  status    = "available"
}

# Lookup a single IP by ID
data "ipzilon_ip_addresses" "single" {
  id = 42
}

output "available_ips" {
  value = [for ip in data.ipzilon_ip_addresses.available.items : ip.address if !ip.is_azure_reserved]
}
