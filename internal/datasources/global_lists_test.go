package datasources

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

func hubJSON(id, site int64, name, space string) string {
	return fmt.Sprintf(`{"id":%d,"site_id":%d,"name":%q,"address_space":%q,"location":null,"description":null}`, id, site, name, space)
}

func scopeJSON(id, hub int64, name, kind, cidr string) string {
	return fmt.Sprintf(`{"id":%d,"hub_id":%d,"parent_id":12,"name":%q,"kind":%q,"cidr":%q,"description":null}`, id, hub, name, kind, cidr)
}

func networkJSON(id, scope int64, name, cidr string) string {
	return fmt.Sprintf(`{"id":%d,"scope_id":%d,"name":%q,"cidr":%q,"description":null}`, id, scope, name, cidr)
}

func subnetJSON(id, network int64, name, cidr string) string {
	return fmt.Sprintf(`{"id":%d,"network_id":%d,"name":%q,"cidr":%q,"description":null,"zone_id":9}`, id, network, name, cidr)
}

func page(items ...string) string {
	return fmt.Sprintf(`{"items":[%s],"total":%d}`, strings.Join(items, ","), len(items))
}

// TestGlobalLookups checks that a lookup without the id of a parent goes to
// the global listing of IPzilon >= 3.2.0, with every filter sent to the
// server, and returns every match.
func TestGlobalLookups(t *testing.T) {
	cases := map[string]struct {
		ds      datasource.DataSource
		cfg     map[string]any
		route   string
		body    string
		wantIDs []int64
	}{
		"networks by cidr": {
			NewNetworksDataSource(), map[string]any{"cidr": "10.0.16.0/22"},
			"/networks/?cidr=10.0.16.0%2F22&limit=1000&offset=0", page(networkJSON(77, 41, "avd", "10.0.16.0/22")), []int64{77},
		},
		"networks without filters": {
			NewNetworksDataSource(), map[string]any{},
			"/networks/?limit=1000&offset=0", page(networkJSON(77, 41, "avd", "10.0.16.0/22"), networkJSON(78, 41, "b", "10.0.20.0/22")), []int64{77, 78},
		},
		"hubs by address_space": {
			NewHubsDataSource(), map[string]any{"address_space": "10.0.0.0/16"},
			"/hubs/?address_space=10.0.0.0%2F16&limit=1000&offset=0", page(hubJSON(3, 1, "hub-weu", "10.0.0.0/16")), []int64{3},
		},
		"hubs without filters": {
			NewHubsDataSource(), map[string]any{},
			"/hubs/?limit=1000&offset=0", page(hubJSON(3, 1, "hub-weu", "10.0.0.0/16")), []int64{3},
		},
		"scopes by kind and name": {
			NewScopesDataSource(), map[string]any{"kind": "project", "name": "avd"},
			"/scopes/?kind=project&name=avd&limit=1000&offset=0", page(scopeJSON(41, 3, "avd", "project", "10.0.16.0/20")), []int64{41},
		},
		"scopes by parent": {
			NewScopesDataSource(), map[string]any{"parent_id": int64(12)},
			"/scopes/?parent_id=12&limit=1000&offset=0", page(scopeJSON(41, 3, "avd", "project", "10.0.16.0/20")), []int64{41},
		},
		"scopes without filters": {
			NewScopesDataSource(), map[string]any{},
			"/scopes/?limit=1000&offset=0", page(scopeJSON(41, 3, "avd", "project", "10.0.16.0/20")), []int64{41},
		},
		"subnets by cidr": {
			NewSubnetsDataSource(), map[string]any{"cidr": "10.0.16.64/26"},
			"/subnets/?cidr=10.0.16.64%2F26&limit=1000&offset=0", page(subnetJSON(512, 77, "snet-hosts", "10.0.16.64/26")), []int64{512},
		},
		"subnets by name and zone": {
			NewSubnetsDataSource(), map[string]any{"name": "snet-hosts", "zone_id": int64(9)},
			"/subnets/?name=snet-hosts&zone_id=9&limit=1000&offset=0", page(subnetJSON(512, 77, "snet-hosts", "10.0.16.64/26")), []int64{512},
		},
		"subnets by name in a network": {
			NewSubnetsDataSource(), map[string]any{"name": "snet-hosts", "network_id": int64(77)},
			"/subnets/?name=snet-hosts&network_id=77&limit=1000&offset=0", page(subnetJSON(512, 77, "snet-hosts", "10.0.16.64/26")), []int64{512},
		},
		"same cidr in two hubs": {
			NewNetworksDataSource(), map[string]any{"cidr": "10.0.16.0/22"},
			"/networks/?cidr=10.0.16.0%2F22&limit=1000&offset=0", page(networkJSON(77, 41, "a", "10.0.16.0/22"), networkJSON(90, 55, "b", "10.0.16.0/22")), []int64{77, 90},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c, calls := fakeAPI(t, "3.2.0", map[string]string{tc.route: tc.body})
			resp := readDataSource(t, tc.ds, c, tc.cfg)
			if resp.Diagnostics.HasError() {
				t.Fatalf("Read: %v", resp.Diagnostics)
			}
			items := itemsOf(t, resp)
			if len(items) != len(tc.wantIDs) {
				t.Fatalf("items = %v, want ids %v", items, tc.wantIDs)
			}
			for i, id := range tc.wantIDs {
				if got := numberOf(t, items[i]["id"]); got != id {
					t.Errorf("items[%d].id = %v, want %d", i, got, id)
				}
			}
			if len(*calls) != 1 {
				t.Errorf("calls = %v, want only %s", *calls, tc.route)
			}
		})
	}
}

// TestParentLookupsUnchanged checks that a lookup with the id of a parent keeps
// using the listing of that parent (same results, also on IPzilon 3.1.x) and
// never a global listing.
func TestParentLookupsUnchanged(t *testing.T) {
	cases := map[string]struct {
		ds     datasource.DataSource
		cfg    map[string]any
		routes map[string]string
	}{
		"hubs by site": {NewHubsDataSource(), map[string]any{"site_id": int64(1), "name": "hub-weu"}, map[string]string{
			"/sites/1/hubs?name=hub-weu&limit=1000&offset=0": page(hubJSON(3, 1, "hub-weu", "10.0.0.0/16")),
		}},
		"scopes by hub with kind": {NewScopesDataSource(), map[string]any{"hub_id": int64(3), "kind": "project"}, map[string]string{
			"/hubs/3/scopes?limit=1000&offset=0": page(scopeJSON(41, 3, "avd", "project", "10.0.16.0/20"), scopeJSON(42, 3, "lz", "landing_zone", "10.0.0.0/20")),
		}},
		"networks by hub": {NewNetworksDataSource(), map[string]any{"hub_id": int64(3), "cidr": "10.0.16.0/22"}, map[string]string{
			"/hubs/3/networks?cidr=10.0.16.0%2F22&limit=1000&offset=0": page(networkJSON(77, 41, "avd", "10.0.16.0/22")),
		}},
		"networks by scope": {NewNetworksDataSource(), map[string]any{"scope_id": int64(41)}, map[string]string{
			"/scopes/41/networks?limit=1000&offset=0": page(networkJSON(77, 41, "avd", "10.0.16.0/22")),
		}},
		"subnets by network": {NewSubnetsDataSource(), map[string]any{"network_id": int64(77)}, map[string]string{
			"/networks/77/subnets?limit=1000&offset=0": page(subnetJSON(512, 77, "snet-hosts", "10.0.16.64/26")),
		}},
		"subnets by zone only": {NewSubnetsDataSource(), map[string]any{"zone_id": int64(9)}, map[string]string{
			"/zones/9": zoneJSON(9, 77, "z", "10.0.16.0/23"),
			"/networks/77/subnets?zone_id=9&limit=1000&offset=0": page(subnetJSON(512, 77, "snet-hosts", "10.0.16.64/26")),
		}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c, calls := fakeAPI(t, "3.1.0", tc.routes)
			resp := readDataSource(t, tc.ds, c, tc.cfg)
			if resp.Diagnostics.HasError() {
				t.Fatalf("Read: %v", resp.Diagnostics)
			}
			if items := itemsOf(t, resp); len(items) != 1 {
				t.Errorf("items = %v, want one", items)
			}
			if len(*calls) != len(tc.routes) {
				t.Errorf("calls = %v", *calls)
			}
		})
	}
}

func TestGlobalLookupsOnOldIPzilonFailWithoutRequests(t *testing.T) {
	cases := map[string]struct {
		ds  datasource.DataSource
		cfg map[string]any
	}{
		"hubs":     {NewHubsDataSource(), map[string]any{"name": "hub-weu"}},
		"scopes":   {NewScopesDataSource(), map[string]any{"kind": "project"}},
		"networks": {NewNetworksDataSource(), map[string]any{"cidr": "10.0.16.0/22"}},
		"subnets":  {NewSubnetsDataSource(), map[string]any{"network_id": int64(77), "name": "x"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c, calls := fakeAPI(t, "3.1.0", nil)
			resp := readDataSource(t, tc.ds, c, tc.cfg)
			if !resp.Diagnostics.HasError() || resp.Diagnostics.Errors()[0].Summary() != "IPzilon version not supported" ||
				!strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "requires IPzilon >= 3.2.0") {
				t.Errorf("diagnostics = %v", resp.Diagnostics)
			}
			if len(*calls) != 0 {
				t.Errorf("no request expected, got %v", *calls)
			}
		})
	}
}

func TestGlobalLookupMethodNotAllowedIsVersionError(t *testing.T) {
	c, _ := fakeAPI(t, "", map[string]string{
		"/networks/?cidr=10.0.16.0%2F22&limit=1000&offset=0": `status:405 {"detail":"Method Not Allowed"}`,
	})
	resp := readDataSource(t, NewNetworksDataSource(), c, map[string]any{"cidr": "10.0.16.0/22"})
	if !resp.Diagnostics.HasError() || resp.Diagnostics.Errors()[0].Summary() != "IPzilon version not supported" ||
		!strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "requires IPzilon >= 3.2.0") {
		t.Errorf("diagnostics = %v", resp.Diagnostics)
	}
}

func TestGlobalLookupParentNotFoundShowsAPIError(t *testing.T) {
	c, _ := fakeAPI(t, "3.2.0", map[string]string{
		"/subnets/?cidr=10.0.16.64%2F26&network_id=999&limit=1000&offset=0": `status:404 {"detail":"Network not found"}`,
	})
	resp := readDataSource(t, NewSubnetsDataSource(), c, map[string]any{"cidr": "10.0.16.64/26", "network_id": int64(999)})
	if !resp.Diagnostics.HasError() || !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "Network not found") {
		t.Errorf("diagnostics = %v", resp.Diagnostics)
	}
}

func TestSubnetsWithoutFiltersStillFails(t *testing.T) {
	c, calls := fakeAPI(t, "3.2.0", nil)
	resp := readDataSource(t, NewSubnetsDataSource(), c, map[string]any{})
	if !resp.Diagnostics.HasError() || resp.Diagnostics.Errors()[0].Summary() != "Missing filter" {
		t.Errorf("diagnostics = %v", resp.Diagnostics)
	}
	if len(*calls) != 0 {
		t.Errorf("calls = %v", *calls)
	}
}
