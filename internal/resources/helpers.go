package resources

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// importByID parses the string import ID to int64 and sets the "id" attribute.
// Use in ImportState for all resources that have a numeric id.
func importByID(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("Expected a numeric resource ID, got %q.", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func strPtr(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	s := v.ValueString()
	return &s
}

func int64Ptr(v types.Int64) *int64 {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	n := v.ValueInt64()
	return &n
}

// cidrPrefixLength extracts the prefix length from a CIDR string (e.g.
// "10.1.4.0/24" -> 24). Used in Read() to repopulate the prefix_length
// attribute of next_subnet/next_network/last_subnet after import or refresh,
// since the API response only returns the resulting cidr, never the
// prefix_length that was used to request it. Without this, prefix_length
// stays unknown after `terraform import` and, being RequiresReplace, forces
// a spurious replacement on the very next plan.
func cidrPrefixLength(cidr string) (int64, error) {
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return 0, err
	}
	ones, _ := network.Mask.Size()
	return int64(ones), nil
}
