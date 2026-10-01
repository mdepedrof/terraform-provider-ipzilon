package resources

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/mdepedrof/terraform-provider-ipzilon/internal/client"
)

func TestZoneNameValidators(t *testing.T) {
	cases := map[string]struct {
		value   types.String
		wantErr bool
	}{
		"lowercase": {types.StringValue("pooled_zone_1"), false},
		"uppercase": {types.StringValue("Pooled"), true},
		"null":      {types.StringNull(), false},
		"unknown":   {types.StringUnknown(), false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			resp := &validator.StringResponse{}
			for _, v := range zoneNameValidators {
				v.ValidateString(context.Background(), validator.StringRequest{Path: path.Root("name"), ConfigValue: tc.value}, resp)
			}
			if resp.Diagnostics.HasError() != tc.wantErr {
				t.Errorf("errors = %v, wantErr %v", resp.Diagnostics, tc.wantErr)
			}
		})
	}
}

func TestRequireZones(t *testing.T) {
	cases := map[string]bool{"3.0.1": false, "3.1.0": true, "": true}
	for version, want := range cases {
		var diags diag.Diagnostics
		got := requireZones(&client.Client{APIVersion: version}, "ipzilon_network_zone", &diags)
		if got != want || diags.HasError() == want {
			t.Errorf("requireZones(%q) = %v (diags %v), want %v", version, got, diags, want)
		}
		if !want && !strings.Contains(diags.Errors()[0].Detail(), "ipzilon_network_zone requires IPzilon >= 3.1.0") {
			t.Errorf("requireZones(%q) detail = %q", version, diags.Errors()[0].Detail())
		}
	}
}
