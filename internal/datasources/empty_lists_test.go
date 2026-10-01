package datasources

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

// TestListsWithoutResultsAreEmpty checks that the listing data sources return
// items = [] (never null) when nothing matches, so that for expressions and
// length() work on them. ipzilon_subnets and ipzilon_network_zones are covered
// by TestSubnetsEmptyIsAnEmptyList and TestNetworkZonesEmptyIsNotAnError.
func TestListsWithoutResultsAreEmpty(t *testing.T) {
	const empty = `{"items":[],"total":0}`
	cases := map[string]struct {
		ds    datasource.DataSource
		cfg   map[string]any
		route string
	}{
		"sites":        {NewSitesDataSource(), map[string]any{"name": "x"}, "/sites/?name=x&limit=1000&offset=0"},
		"hubs":         {NewHubsDataSource(), map[string]any{"site_id": int64(1)}, "/sites/1/hubs?limit=1000&offset=0"},
		"scopes":       {NewScopesDataSource(), map[string]any{"hub_id": int64(1)}, "/hubs/1/scopes?limit=1000&offset=0"},
		"scopes kind":  {NewScopesDataSource(), map[string]any{"hub_id": int64(1), "kind": "project"}, "/hubs/1/scopes?limit=1000&offset=0"},
		"networks":     {NewNetworksDataSource(), map[string]any{"scope_id": int64(1)}, "/scopes/1/networks?limit=1000&offset=0"},
		"ip addresses": {NewIPAddressesDataSource(), map[string]any{"subnet_id": int64(1), "status": "reserved"}, "/subnets/1/ips?status=reserved&limit=1000&offset=0"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c, _ := fakeAPI(t, "3.2.0", map[string]string{tc.route: empty})
			resp := readDataSource(t, tc.ds, c, tc.cfg)
			if resp.Diagnostics.HasError() {
				t.Fatalf("Read: %v", resp.Diagnostics)
			}
			if itemsAreNull(t, resp) {
				t.Fatal("items is null, want an empty list")
			}
			if items := itemsOf(t, resp); len(items) != 0 {
				t.Errorf("items = %v, want empty", items)
			}
		})
	}
}
