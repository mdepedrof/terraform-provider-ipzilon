package datasources

import (
	"strings"
	"testing"
)

const subnetsInZone12 = `{"items":[{"id":40,"network_id":7,"name":"hp","cidr":"10.0.16.0/28","description":null,"zone_id":12}],"total":1}`

func TestSubnetsByZoneResolvesNetwork(t *testing.T) {
	c, calls := fakeAPI(t, "3.1.0", map[string]string{
		"/zones/12": zoneJSON(12, 7, "pooled_zone_1", "10.0.16.0/23"),
		"/networks/7/subnets?zone_id=12&limit=1000&offset=0": subnetsInZone12,
	})
	resp := readDataSource(t, NewSubnetsDataSource(), c, map[string]any{"zone_id": int64(12)})
	if resp.Diagnostics.HasError() {
		t.Fatalf("Read: %v", resp.Diagnostics)
	}
	items := itemsOf(t, resp)
	if len(items) != 1 || numberOf(t, items[0]["zone_id"]) != int64(12) {
		t.Errorf("items = %v", items)
	}
	if len(*calls) != 2 {
		t.Errorf("calls = %v", *calls)
	}
}

func TestSubnetsByNetworkAndZoneSkipsZoneLookup(t *testing.T) {
	c, calls := fakeAPI(t, "3.1.0", map[string]string{
		"/networks/7/subnets?zone_id=12&limit=1000&offset=0": subnetsInZone12,
	})
	resp := readDataSource(t, NewSubnetsDataSource(), c, map[string]any{"network_id": int64(7), "zone_id": int64(12)})
	if resp.Diagnostics.HasError() {
		t.Fatalf("Read: %v", resp.Diagnostics)
	}
	if len(*calls) != 1 {
		t.Errorf("calls = %v, want only the listing", *calls)
	}
}

func TestSubnetsNoZone(t *testing.T) {
	c, _ := fakeAPI(t, "3.1.0", map[string]string{
		"/networks/7/subnets?no_zone=true&limit=1000&offset=0": `{"items":[{"id":41,"network_id":7,"name":"out","cidr":"10.0.20.0/28","description":null,"zone_id":null}],"total":1}`,
	})
	resp := readDataSource(t, NewSubnetsDataSource(), c, map[string]any{"network_id": int64(7), "no_zone": true})
	if resp.Diagnostics.HasError() {
		t.Fatalf("Read: %v", resp.Diagnostics)
	}
	items := itemsOf(t, resp)
	if len(items) != 1 || numberOf(t, items[0]["zone_id"]) != nil {
		t.Errorf("items = %v", items)
	}
}

// TestSubnetsByNetworkUnchanged: the existing filter keeps its request and
// works against IPzilon 3.0.x (no zone_id in the response).
func TestSubnetsByNetworkUnchanged(t *testing.T) {
	c, _ := fakeAPI(t, "3.0.1", map[string]string{
		"/networks/7/subnets?limit=1000&offset=0": `{"items":[{"id":40,"network_id":7,"name":"hp","cidr":"10.0.16.0/28","description":null}],"total":1}`,
	})
	resp := readDataSource(t, NewSubnetsDataSource(), c, map[string]any{"network_id": int64(7)})
	if resp.Diagnostics.HasError() {
		t.Fatalf("Read: %v", resp.Diagnostics)
	}
	if items := itemsOf(t, resp); len(items) != 1 || numberOf(t, items[0]["zone_id"]) != nil {
		t.Errorf("items = %v", items)
	}
}

func TestSubnetsZoneFilterErrors(t *testing.T) {
	cases := map[string]struct {
		version string
		cfg     map[string]any
		want    string
	}{
		"zone_id and no_zone":     {"3.1.0", map[string]any{"network_id": int64(7), "zone_id": int64(12), "no_zone": true}, "mutually exclusive"},
		"no_zone without network": {"3.1.0", map[string]any{"no_zone": true}, "no_zone requires network_id"},
		"id with zone_id":         {"3.1.0", map[string]any{"id": int64(40), "zone_id": int64(12)}, "not both"},
		"no filter":               {"3.1.0", map[string]any{}, "Provide either id or a parent filter"},
		"old IPzilon":             {"3.0.1", map[string]any{"zone_id": int64(12)}, "requires IPzilon >= 3.1.0"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c, calls := fakeAPI(t, tc.version, nil)
			resp := readDataSource(t, NewSubnetsDataSource(), c, tc.cfg)
			if !resp.Diagnostics.HasError() || !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), tc.want) {
				t.Errorf("diagnostics = %v, want %q", resp.Diagnostics, tc.want)
			}
			if len(*calls) != 0 {
				t.Errorf("no request expected, got %v", *calls)
			}
		})
	}
}

// TestSubnetsEmptyIsAnEmptyList: a filter without matches returns an empty
// list, not null, so `for` expressions over items keep working.
func TestSubnetsEmptyIsAnEmptyList(t *testing.T) {
	c, _ := fakeAPI(t, "3.1.0", map[string]string{
		"/networks/7/subnets?no_zone=true&limit=1000&offset=0": `{"items":[],"total":0}`,
	})
	resp := readDataSource(t, NewSubnetsDataSource(), c, map[string]any{"network_id": int64(7), "no_zone": true})
	if resp.Diagnostics.HasError() {
		t.Fatalf("Read: %v", resp.Diagnostics)
	}
	if items := itemsOf(t, resp); items == nil || len(items) != 0 {
		t.Errorf("items = %v, want an empty list", items)
	}
}
