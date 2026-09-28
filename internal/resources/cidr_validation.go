package resources

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// maxCIDRSize is the largest block (smallest prefix) IPzilon >= 3.0 accepts
// for hubs, scopes, networks and subnets.
const maxCIDRSize = 8

// checkCIDRChange mirrors IPzilon's size and IPv6 rules so they fail in plan.
// Like the API, it only checks a value being created or changed: existing
// objects larger than /8 keep working while their CIDR stays the same.
// Unparseable values are left for the API to reject.
func checkCIDRChange(prior, planned types.String, ipv4Only bool) (summary, detail string, bad bool) {
	if planned.IsNull() || planned.IsUnknown() || planned.Equal(prior) {
		return "", "", false
	}
	prefix, err := netip.ParsePrefix(planned.ValueString())
	if err != nil {
		return "", "", false
	}
	if ipv4Only && !prefix.Addr().Is4() {
		return "IPv6 not supported", "IPv6 subnets are not supported", true
	}
	if prefix.Addr().Is4() && prefix.Bits() < maxCIDRSize {
		return "CIDR too large", fmt.Sprintf("/%d is too large: the maximum size is /%d", prefix.Bits(), maxCIDRSize), true
	}
	return "", "", false
}

// cidrLimits returns a plan modifier that applies checkCIDRChange to a CIDR
// attribute. It is a plan modifier rather than a validator because it needs
// the prior state to let unchanged legacy values through.
func cidrLimits(ipv4Only bool) planmodifier.String {
	return cidrLimitsModifier{ipv4Only: ipv4Only}
}

type cidrLimitsModifier struct{ ipv4Only bool }

func (m cidrLimitsModifier) Description(_ context.Context) string {
	if m.ipv4Only {
		return "CIDR must be IPv4 and /8 or smaller when created or changed"
	}
	return "CIDR must be /8 or smaller when created or changed"
}

func (m cidrLimitsModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m cidrLimitsModifier) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if summary, detail, bad := checkCIDRChange(req.StateValue, req.PlanValue, m.ipv4Only); bad {
		resp.Diagnostics.AddAttributeError(req.Path, summary, detail)
	}
}
