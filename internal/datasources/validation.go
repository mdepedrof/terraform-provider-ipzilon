package datasources

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// networkCIDR checks that s is a CIDR without host bits, as the global
// listings of IPzilon >= 3.2.0 require, and returns it in canonical form. The
// error suggests the network address when s has host bits.
func networkCIDR(s string) (string, error) {
	p, err := netip.ParsePrefix(s)
	if err != nil {
		return "", fmt.Errorf("'%s' is not a valid CIDR", s)
	}
	if m := p.Masked(); m != p {
		return "", fmt.Errorf("'%s' is not a network address; did you mean '%s'?", s, m)
	}
	return p.String(), nil
}

// validateGlobalCIDR adds an attribute error when the CIDR filter attr of a
// global lookup is not a network address. Null or unknown values are skipped:
// the API checks them again when it is called.
func validateGlobalCIDR(ctx context.Context, cfg tfsdk.Config, attr string, diags *diag.Diagnostics) {
	var v types.String
	diags.Append(cfg.GetAttribute(ctx, path.Root(attr), &v)...)
	if v.IsNull() || v.IsUnknown() {
		return
	}
	if _, err := networkCIDR(v.ValueString()); err != nil {
		diags.AddAttributeError(path.Root(attr), "Invalid CIDR", err.Error())
	}
}
