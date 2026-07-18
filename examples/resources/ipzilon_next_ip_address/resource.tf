# Atomically reserve the next available IP in a subnet.
# The address is assigned by the server; use hostname as the semantic name.
resource "ipzilon_next_ip_address" "app_vm" {
  subnet_id   = ipzilon_next_subnet.example.id
  hostname    = "app-vm-01"
  description = "Application VM"
}

# Available attributes after apply:
# ipzilon_next_ip_address.app_vm.address          → "10.0.1.4"
# ipzilon_next_ip_address.app_vm.hostname         → "app-vm-01"
# ipzilon_next_ip_address.app_vm.status           → "used"
# ipzilon_next_ip_address.app_vm.is_azure_reserved → false
