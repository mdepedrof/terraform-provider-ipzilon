package resources

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/mdepedrof/terraform-provider-ipzilon/internal/client"
)

// zoneNameValidators rejects uppercase zone names in plan: IPzilon stores
// every string in lowercase, so an uppercase name would show a diff on every
// plan.
var zoneNameValidators = []validator.String{
	stringvalidator.RegexMatches(regexp.MustCompile(`^[^A-Z]*$`), "must be lowercase — the server normalizes all strings"),
}

// prefixLengthValidators mirrors the 8..32 range IPzilon accepts for an
// automatically allocated zone, so a typo fails in plan.
var prefixLengthValidators = []validator.Int64{int64validator.Between(8, 32)}

// requireZones reports whether the server supports network zones (IPzilon >=
// 3.1.0), adding an error that names feature when it does not. It makes no
// request: the version was read from /health when the provider was configured.
func requireZones(c *client.Client, feature string, diags *diag.Diagnostics) bool {
	if err := c.RequireAPIVersion(client.MinZonesAPIVersion, feature); err != nil {
		diags.AddError("IPzilon version not supported", err.Error())
		return false
	}
	return true
}

// zoneIDAttribute is the zone_id of the subnet resources: Optional + Computed
// and never RequiresReplace, since the zone of a subnet is computed by IPzilon
// from CIDR containment and can change when zones are created, resized or
// deleted. modifier keeps the planned value stable when it is not configured.
func zoneIDAttribute(description string, modifier planmodifier.Int64) schema.Int64Attribute {
	return schema.Int64Attribute{
		Optional:      true,
		Computed:      true,
		Description:   description,
		PlanModifiers: []planmodifier.Int64{modifier},
	}
}

// configZoneID returns the zone_id written in the configuration, nil when it
// is not set. A value that only comes from the state (UseStateForUnknown) is
// never sent: it may be a zone deleted since the last apply.
func configZoneID(ctx context.Context, cfg tfsdk.Config, diags *diag.Diagnostics) *int64 {
	var v types.Int64
	diags.Append(cfg.GetAttribute(ctx, path.Root("zone_id"), &v)...)
	return int64Ptr(v)
}

// modifyPlanZones reports an IPzilon older than 3.1.0 already in plan when
// the configuration of a subnet resource sets zone_id.
func modifyPlanZones(ctx context.Context, c *client.Client, feature string, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if c == nil || req.Plan.Raw.IsNull() {
		return
	}
	if configZoneID(ctx, req.Config, &resp.Diagnostics) != nil {
		requireZones(c, feature, &resp.Diagnostics)
	}
}

// checkZoneApplied guards against a server that ignores zone_id (IPzilon
// 3.0.x reporting an unknown version): when a zone was requested and the
// subnet returned is not in it, it adds a version error and, if rollback is
// true (Create), deletes the subnet just created so it never stays outside
// the requested zone. It returns false when it added the error.
func checkZoneApplied(ctx context.Context, c *client.Client, feature string, requested *int64, got client.Subnet, rollback bool, diags *diag.Diagnostics) bool {
	if requested == nil || (got.ZoneID != nil && *got.ZoneID == *requested) {
		return true
	}
	if rollback {
		if err := c.Delete(ctx, fmt.Sprintf("/subnets/%d", got.ID)); err != nil && !client.IsNotFound(err) {
			diags.AddError("Rollback of subnet failed", fmt.Sprintf("subnet %d was created outside zone %d and could not be deleted: %s", got.ID, *requested, err))
		}
	}
	diags.AddError("IPzilon version not supported", fmt.Sprintf("%s requires IPzilon >= %s: the server ignored zone_id", feature, client.MinZonesAPIVersion))
	return false
}

// zoneIDFollowsCIDR is the zone_id plan modifier of ipzilon_subnet: when
// zone_id is not configured it keeps the state value only while the CIDR
// does not change. A new CIDR may move the subnet into or out of a zone, so
// the value is left unknown (UseStateForUnknown would make Terraform fail
// with "inconsistent result after apply").
func zoneIDFollowsCIDR() planmodifier.Int64 { return zoneIDFollowsCIDRModifier{} }

type zoneIDFollowsCIDRModifier struct{}

func (zoneIDFollowsCIDRModifier) Description(_ context.Context) string {
	return "keeps the zone computed by IPzilon while the CIDR does not change"
}

func (m zoneIDFollowsCIDRModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (zoneIDFollowsCIDRModifier) PlanModifyInt64(ctx context.Context, req planmodifier.Int64Request, resp *planmodifier.Int64Response) {
	// Same guards as UseStateForUnknown: nothing to keep on create, when the
	// value is known (configured) or while the configuration is unknown.
	if req.State.Raw.IsNull() || !req.PlanValue.IsUnknown() || req.ConfigValue.IsUnknown() {
		return
	}
	var planned, prior types.String
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("cidr"), &planned)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("cidr"), &prior)...)
	if planned.Equal(prior) {
		resp.PlanValue = req.StateValue
	}
}

// zoneRouteMissing reports whether err is the generic 404 of a route that
// does not exist, i.e. an IPzilon older than 3.1.0 whose version is unknown
// (0.0.0-dev). IPzilon 3.1.0 answers a missing zone or network with a
// specific detail ("Zone not found", "Network not found").
func zoneRouteMissing(err error) bool {
	var apiErr *client.APIError
	return errors.As(err, &apiErr) && apiErr.Code == 404 && apiErr.Message == "Not Found"
}

// zoneAPIError adds the error of a zone request, replacing the generic 404 of
// an IPzilon without zones with the version error.
func zoneAPIError(diags *diag.Diagnostics, summary, feature string, err error) {
	if zoneRouteMissing(err) {
		diags.AddError("IPzilon version not supported", fmt.Sprintf("%s requires IPzilon >= %s: the server has no zone endpoints (%s)", feature, client.MinZonesAPIVersion, err))
		return
	}
	diags.AddError(summary, err.Error())
}
