package datasources

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/mdepedrof/terraform-provider-ipzilon/internal/client"
)

// TestIPToItem_NullID verifies that free addresses (no stored record since
// IPzilon 3.0) map to a null id instead of a fake 0.
func TestIPToItem_NullID(t *testing.T) {
	free := ipToItem(client.IPAddress{ID: nil, SubnetID: 5, Address: "10.0.1.17", Status: "available"})
	if !free.ID.IsNull() {
		t.Errorf("free.ID = %v, want null", free.ID)
	}

	id := int64(7)
	used := ipToItem(client.IPAddress{ID: &id, SubnetID: 5, Address: "10.0.1.18", Status: "used"})
	if !used.ID.Equal(types.Int64Value(7)) {
		t.Errorf("used.ID = %v, want 7", used.ID)
	}
}

func TestSubnetIPsURL(t *testing.T) {
	status := "available"
	cases := []struct {
		status *string
		want   string
	}{
		{nil, "/subnets/5/ips"},
		{&status, "/subnets/5/ips?status=available"},
	}
	for _, c := range cases {
		if got := subnetIPsURL(5, c.status); got != c.want {
			t.Errorf("subnetIPsURL(5, %v) = %q, want %q", c.status, got, c.want)
		}
	}
}
