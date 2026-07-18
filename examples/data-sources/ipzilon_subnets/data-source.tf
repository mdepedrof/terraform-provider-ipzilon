# List all subnets in a network
data "ipzilon_subnets" "all" {
  network_id = 1
}

output "subnet_cidrs" {
  value = [for s in data.ipzilon_subnets.all.items : s.cidr]
}
