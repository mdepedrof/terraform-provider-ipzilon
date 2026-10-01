package datasources

import (
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

func TestGlobalLookupsValidateConfig(t *testing.T) {
	cases := map[string]struct {
		ds         datasource.DataSource
		cfg        map[string]any
		want       string // summary of the expected error, "" for none
		wantDetail string
	}{
		"networks, host bits":            {NewNetworksDataSource(), map[string]any{"cidr": "10.0.16.5/22"}, "Invalid CIDR", "did you mean '10.0.16.0/22'?"},
		"networks, network address":      {NewNetworksDataSource(), map[string]any{"cidr": "10.0.16.0/22"}, "", ""},
		"networks, host bits with scope": {NewNetworksDataSource(), map[string]any{"cidr": "10.0.16.5/22", "scope_id": int64(1)}, "", ""},
		"networks, host bits with hub":   {NewNetworksDataSource(), map[string]any{"cidr": "10.0.16.5/22", "hub_id": int64(1)}, "", ""},
		"networks, unknown scope":        {NewNetworksDataSource(), map[string]any{"cidr": "10.0.16.5/22", "scope_id": unknown}, "", ""},
		"networks, unknown cidr":         {NewNetworksDataSource(), map[string]any{"cidr": unknown}, "", ""},
		"hubs, not a cidr":               {NewHubsDataSource(), map[string]any{"address_space": "foo"}, "Invalid CIDR", "'foo' is not a valid CIDR"},
		"hubs, not a cidr with site":     {NewHubsDataSource(), map[string]any{"address_space": "foo", "site_id": int64(1)}, "", ""},
		"scopes, host bits":              {NewScopesDataSource(), map[string]any{"cidr": "10.0.16.5/20"}, "Invalid CIDR", "did you mean '10.0.16.0/20'?"},
		"scopes, host bits with hub":     {NewScopesDataSource(), map[string]any{"cidr": "10.0.16.5/20", "hub_id": int64(1)}, "", ""},
		"scopes, root_only without hub":  {NewScopesDataSource(), map[string]any{"root_only": true}, "Missing filter", "root_only requires hub_id"},
		"scopes, root_only with hub":     {NewScopesDataSource(), map[string]any{"root_only": true, "hub_id": int64(1)}, "", ""},
		"scopes, root_only false":        {NewScopesDataSource(), map[string]any{"root_only": false}, "", ""},
		"subnets, host bits":             {NewSubnetsDataSource(), map[string]any{"cidr": "10.0.16.65/26"}, "Invalid CIDR", "did you mean '10.0.16.64/26'?"},
		"subnets, host bits in network":  {NewSubnetsDataSource(), map[string]any{"cidr": "10.0.16.65/26", "network_id": int64(7)}, "Invalid CIDR", ""},
		"subnets, no_zone with name":     {NewSubnetsDataSource(), map[string]any{"network_id": int64(7), "no_zone": true, "name": "x"}, "Conflicting filters", "no_zone cannot be combined with name or cidr"},
		"subnets, no_zone alone":         {NewSubnetsDataSource(), map[string]any{"network_id": int64(7), "no_zone": true}, "", ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			resp := validateDataSource(t, tc.ds, tc.cfg)
			if tc.want == "" {
				if resp.Diagnostics.HasError() {
					t.Errorf("unexpected error: %v", resp.Diagnostics)
				}
				return
			}
			if !resp.Diagnostics.HasError() {
				t.Fatalf("want %q, got no error", tc.want)
			}
			d := resp.Diagnostics.Errors()[0]
			if d.Summary() != tc.want || !strings.Contains(d.Detail(), tc.wantDetail) {
				t.Errorf("diagnostic = %s: %s, want %s: %s", d.Summary(), d.Detail(), tc.want, tc.wantDetail)
			}
		})
	}
}
