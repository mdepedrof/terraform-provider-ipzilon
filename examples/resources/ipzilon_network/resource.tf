resource "ipzilon_network" "example" {
  scope_id    = ipzilon_scope.child.id
  name        = "spoke-app"
  cidr        = "10.0.1.0/24"
  description = "Application Spoke Network"
}
