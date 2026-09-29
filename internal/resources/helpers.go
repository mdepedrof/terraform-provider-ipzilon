package resources

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/mdepedrof/terraform-provider-ipzilon/internal/client"
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

// ipIDValue converts the id of an IP record returned by the API. Since
// IPzilon 3.0 free addresses come with a null id; the endpoints the resources
// use (register, reserve-ip, GET/PATCH /ips/{id}) always return a stored
// record, so a null id there is an unexpected response.
func ipIDValue(id *int64) (types.Int64, error) {
	if id == nil {
		return types.Int64Null(), errors.New("Unexpected response: IP address without id")
	}
	return types.Int64Value(*id), nil
}

// releaseIP frees an address managed by ipzilon_ip_address or
// ipzilon_next_ip_address. In IPzilon 3.0 DELETE /ips/{id} releases the
// address (its record is deleted and the address becomes free again). A 404
// means it was already released outside Terraform.
func releaseIP(ctx context.Context, c *client.Client, id int64) error {
	if err := c.Delete(ctx, fmt.Sprintf("/ips/%d", id)); err != nil && !client.IsNotFound(err) {
		return err
	}
	return nil
}

// ipStatusValidator restricts the status of the address resources to used or
// reserved: a free address has no record since IPzilon 3.0, so releasing an
// address means destroying the resource.
var ipStatusValidator = stringvalidator.OneOf("used", "reserved")

// ipAddressValidator checks that a string is a valid IPv4 or IPv6 address, so
// a typo fails in plan instead of apply (shape validation, Principle I).
type ipAddressValidator struct{}

func (ipAddressValidator) Description(_ context.Context) string {
	return "value must be a valid IP address"
}

func (v ipAddressValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (ipAddressValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if _, err := netip.ParseAddr(req.ConfigValue.ValueString()); err != nil {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid IP address", fmt.Sprintf("%q is not a valid IP address.", req.ConfigValue.ValueString()))
	}
}
