package datasources

import (
	"errors"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"

	"github.com/mdepedrof/terraform-provider-ipzilon/internal/client"
)

func TestNetworkCIDR(t *testing.T) {
	cases := []struct {
		in, want, wantErr string
	}{
		{in: "10.0.16.0/22", want: "10.0.16.0/22"},
		{in: "10.0.16.5/22", wantErr: "is not a network address; did you mean '10.0.16.0/22'?"},
		{in: "foo", wantErr: "'foo' is not a valid CIDR"},
		{in: "10.0.16.0", wantErr: "is not a valid CIDR"},
		{in: "2001:db8::/32", want: "2001:db8::/32"},
		{in: "2001:db8::1/32", wantErr: "did you mean '2001:db8::/32'?"},
	}
	for _, tc := range cases {
		got, err := networkCIDR(tc.in)
		if tc.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("networkCIDR(%q) error = %v, want %q", tc.in, err, tc.wantErr)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("networkCIDR(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}
}

func TestRequireGlobalLists(t *testing.T) {
	for v, want := range map[string]bool{"3.1.0": false, "3.2.0": true, "": true} {
		var diags diag.Diagnostics
		got := requireGlobalLists(&client.Client{APIVersion: v}, "ipzilon_hubs without site_id", &diags)
		if got != want || diags.HasError() == want {
			t.Errorf("version %q: got %v, diags %v", v, got, diags)
		}
		if !want && !strings.Contains(diags.Errors()[0].Detail(), "ipzilon_hubs without site_id requires IPzilon >= 3.2.0") {
			t.Errorf("version %q: detail %q", v, diags.Errors()[0].Detail())
		}
	}
}

func TestGlobalListError(t *testing.T) {
	var diags diag.Diagnostics
	globalListError(&diags, "List hubs failed", "ipzilon_hubs without site_id", &client.APIError{Code: 405, Message: "Method Not Allowed"})
	if diags.Errors()[0].Summary() != "IPzilon version not supported" || !strings.Contains(diags.Errors()[0].Detail(), "requires IPzilon >= 3.2.0") {
		t.Errorf("405: %v", diags)
	}

	diags = nil
	globalListError(&diags, "List scopes failed", "x", &client.APIError{Code: 404, Message: "Hub not found"})
	if diags.Errors()[0].Summary() != "List scopes failed" || !strings.Contains(diags.Errors()[0].Detail(), "Hub not found") {
		t.Errorf("404: %v", diags)
	}

	diags = nil
	globalListError(&diags, "List failed", "x", errors.New("boom"))
	if diags.Errors()[0].Summary() != "List failed" {
		t.Errorf("other: %v", diags)
	}
}
