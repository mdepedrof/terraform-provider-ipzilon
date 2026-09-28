package resources

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestCidrPrefixLength(t *testing.T) {
	cases := []struct {
		name    string
		cidr    string
		want    int64
		wantErr bool
	}{
		{name: "/24", cidr: "10.1.4.0/24", want: 24},
		{name: "/16", cidr: "10.0.0.0/16", want: 16},
		{name: "/29 non-aligned host bits ignored by ParseCIDR mask", cidr: "10.1.4.16/29", want: 29},
		{name: "invalid cidr", cidr: "not-a-cidr", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := cidrPrefixLength(tc.cidr)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("cidrPrefixLength(%q) expected error, got nil", tc.cidr)
				}
				return
			}
			if err != nil {
				t.Fatalf("cidrPrefixLength(%q) unexpected error: %v", tc.cidr, err)
			}
			if got != tc.want {
				t.Errorf("cidrPrefixLength(%q) = %d, want %d", tc.cidr, got, tc.want)
			}
		})
	}
}

func TestIPIDValue(t *testing.T) {
	id := int64(7)
	got, err := ipIDValue(&id)
	if err != nil || got.ValueInt64() != 7 {
		t.Errorf("ipIDValue(&7) = %v, %v; want 7, nil", got, err)
	}
	if _, err := ipIDValue(nil); err == nil {
		t.Error("ipIDValue(nil) expected error, got nil")
	}
}

func TestIPAddressValidator(t *testing.T) {
	for value, wantErr := range map[string]bool{"10.0.1.5": false, "fd00::1": false, "10.0.1.256": true, "10.0.1.0/24": true, "web": true} {
		resp := &validator.StringResponse{}
		ipAddressValidator{}.ValidateString(context.Background(), validator.StringRequest{
			Path:        path.Root("address"),
			ConfigValue: types.StringValue(value),
		}, resp)
		if resp.Diagnostics.HasError() != wantErr {
			t.Errorf("%q: error = %v, want %v", value, resp.Diagnostics.HasError(), wantErr)
		}
	}
}
