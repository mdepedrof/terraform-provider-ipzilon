# List the available IPs in a subnet.
# Set status whenever possible: without it the whole subnet is listed
# (e.g. 65,536 items for a /16, fetched in pages of 1000).
data "ipzilon_ip_addresses" "available" {
  subnet_id = 1
  status    = "available"
}

# Lookup a single IP by ID. Only occupied, reserved or annotated addresses have
# an ID (free addresses are listed with id = null), and a released address that
# is occupied again gets a new ID.
data "ipzilon_ip_addresses" "single" {
  id = 42
}

# Lookup one address of a subnet, occupied or free. The address is the stable
# way to refer to an IP: its id changes when it is released and occupied
# again. A free address is returned with id = null and status = "available";
# an address outside the subnet is an error.
data "ipzilon_ip_addresses" "gw" {
  subnet_id = 1
  address   = "10.0.1.17"
}

output "gw_status" {
  value = one(data.ipzilon_ip_addresses.gw.items).status
}

output "available_ips" {
  value = [for ip in data.ipzilon_ip_addresses.available.items : ip.address if !ip.is_azure_reserved]
}
