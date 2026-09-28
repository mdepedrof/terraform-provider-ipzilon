package resources

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestCheckCIDRChange(t *testing.T) {
	cases := []struct {
		name        string
		prior       types.String
		planned     types.String
		ipv4Only    bool
		wantSummary string
		wantDetail  string
	}{
		{name: "new /7", prior: types.StringNull(), planned: types.StringValue("10.0.0.0/7"),
			wantSummary: "CIDR too large", wantDetail: "/7 is too large: the maximum size is /8"},
		{name: "new /8", prior: types.StringNull(), planned: types.StringValue("10.0.0.0/8")},
		{name: "change to /6", prior: types.StringValue("10.0.0.0/16"), planned: types.StringValue("8.0.0.0/6"),
			wantSummary: "CIDR too large", wantDetail: "/6 is too large: the maximum size is /8"},
		{name: "ipv6 subnet", prior: types.StringNull(), planned: types.StringValue("fd00::/64"), ipv4Only: true,
			wantSummary: "IPv6 not supported", wantDetail: "IPv6 subnets are not supported"},
		{name: "ipv6 elsewhere", prior: types.StringNull(), planned: types.StringValue("fd00::/64")},
		{name: "unchanged legacy /7", prior: types.StringValue("10.0.0.0/7"), planned: types.StringValue("10.0.0.0/7")},
		{name: "unknown", prior: types.StringNull(), planned: types.StringUnknown()},
		{name: "null", prior: types.StringValue("10.0.0.0/7"), planned: types.StringNull()},
		{name: "unparseable left to API", prior: types.StringNull(), planned: types.StringValue("not-a-cidr")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			summary, detail, bad := checkCIDRChange(tc.prior, tc.planned, tc.ipv4Only)
			if bad != (tc.wantSummary != "") || summary != tc.wantSummary || detail != tc.wantDetail {
				t.Errorf("got (%q, %q, %v), want (%q, %q)", summary, detail, bad, tc.wantSummary, tc.wantDetail)
			}
		})
	}
}
