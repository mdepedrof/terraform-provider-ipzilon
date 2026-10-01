package datasources

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"

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
	address := "10.0.1.17"
	cases := []struct {
		status, address *string
		want            string
	}{
		{nil, nil, "/subnets/5/ips"},
		{&status, nil, "/subnets/5/ips?status=available"},
		{nil, &address, "/subnets/5/ips?address=10.0.1.17"},
	}
	for _, c := range cases {
		if got := subnetIPsURL(5, c.status, c.address); got != c.want {
			t.Errorf("subnetIPsURL(5, %v, %v) = %q, want %q", c.status, c.address, got, c.want)
		}
	}
}

func ipJSON(id, address, status string) string {
	return fmt.Sprintf(`{"id":%s,"subnet_id":42,"address":%q,"status":%q,"is_azure_reserved":false,"hostname":null,"description":null}`, id, address, status)
}

func TestIPAddressesByAddress(t *testing.T) {
	cases := map[string]struct {
		address, body string
		wantID        any
		wantStatus    string
	}{
		"occupied": {"10.0.1.17", ipJSON("7", "10.0.1.17", "used"), int64(7), "used"},
		"free":     {"10.0.1.20", ipJSON("null", "10.0.1.20", "available"), nil, "available"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c, calls := fakeAPI(t, "3.2.0", map[string]string{
				"/subnets/42/ips?address=" + tc.address + "&limit=1000&offset=0": `{"items":[` + tc.body + `],"total":1}`,
			})
			resp := readDataSource(t, NewIPAddressesDataSource(), c, map[string]any{"subnet_id": int64(42), "address": tc.address})
			if resp.Diagnostics.HasError() {
				t.Fatalf("Read: %v", resp.Diagnostics)
			}
			items := itemsOf(t, resp)
			if len(items) != 1 {
				t.Fatalf("items = %v, want one", items)
			}
			if got := numberOf(t, items[0]["id"]); got != tc.wantID {
				t.Errorf("id = %v, want %v", got, tc.wantID)
			}
			if !items[0]["status"].Equal(tftypes.NewValue(tftypes.String, tc.wantStatus)) {
				t.Errorf("status = %v, want %s", items[0]["status"], tc.wantStatus)
			}
			if len(*calls) != 1 {
				t.Errorf("calls = %v", *calls)
			}
		})
	}
}

func TestIPAddressesByAddressOutsideSubnetFails(t *testing.T) {
	c, _ := fakeAPI(t, "3.2.0", map[string]string{
		"/subnets/42/ips?address=10.0.1.99&limit=1000&offset=0": `{"items":[],"total":0}`,
	})
	resp := readDataSource(t, NewIPAddressesDataSource(), c, map[string]any{"subnet_id": int64(42), "address": "10.0.1.99"})
	if !resp.Diagnostics.HasError() {
		t.Fatal("want an error")
	}
	d := resp.Diagnostics.Errors()[0]
	if d.Summary() != "IP address not found" || !strings.Contains(d.Detail(), "10.0.1.99") || !strings.Contains(d.Detail(), "subnet 42") {
		t.Errorf("diagnostic = %s: %s", d.Summary(), d.Detail())
	}
}

func TestIPAddressesValidateConfig(t *testing.T) {
	cases := map[string]struct {
		cfg  map[string]any
		want string // summary of the expected error, "" for none
	}{
		"ipv4":              {map[string]any{"subnet_id": int64(42), "address": "10.0.1.17"}, ""},
		"ipv6":              {map[string]any{"subnet_id": int64(42), "address": "2001:db8::10"}, ""},
		"unknown address":   {map[string]any{"subnet_id": int64(42), "address": unknown}, ""},
		"no address":        {map[string]any{"subnet_id": int64(42), "status": "used"}, ""},
		"bad address":       {map[string]any{"subnet_id": int64(42), "address": "nope"}, "Invalid address"},
		"cidr as address":   {map[string]any{"subnet_id": int64(42), "address": "10.0.1.0/24"}, "Invalid address"},
		"without subnet_id": {map[string]any{"address": "10.0.1.17"}, "Missing filter"},
		"with status":       {map[string]any{"subnet_id": int64(42), "address": "10.0.1.17", "status": "used"}, "Conflicting filters"},
		"with id":           {map[string]any{"subnet_id": int64(42), "address": "10.0.1.17", "id": int64(7)}, "Conflicting filters"},
		"unknown subnet ok": {map[string]any{"subnet_id": unknown, "address": "10.0.1.17"}, ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			resp := validateDataSource(t, NewIPAddressesDataSource(), tc.cfg)
			if tc.want == "" {
				if resp.Diagnostics.HasError() {
					t.Errorf("unexpected error: %v", resp.Diagnostics)
				}
				return
			}
			if !resp.Diagnostics.HasError() || resp.Diagnostics.Errors()[0].Summary() != tc.want {
				t.Errorf("diagnostics = %v, want %q", resp.Diagnostics, tc.want)
			}
		})
	}
}
