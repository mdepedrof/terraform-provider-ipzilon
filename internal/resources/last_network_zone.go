package resources

import "github.com/hashicorp/terraform-plugin-framework/resource"

// NewLastNetworkZoneResource is ipzilon_last_network_zone: the same resource
// as ipzilon_next_network_zone, allocating the last empty block of the
// network instead of the first one.
func NewLastNetworkZoneResource() resource.Resource { return &allocZoneResource{direction: "last"} }
