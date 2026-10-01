package resources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/mdepedrof/terraform-provider-ipzilon/internal/client"
)

var _ resource.Resource = &HubResource{}
var _ resource.ResourceWithImportState = &HubResource{}
var _ resource.ResourceWithModifyPlan = &HubResource{}

type HubResource struct{ client *client.Client }

func NewHubResource() resource.Resource { return &HubResource{} }

type hubModel struct {
	ID           types.Int64  `tfsdk:"id"`
	SiteID       types.Int64  `tfsdk:"site_id"`
	Name         types.String `tfsdk:"name"`
	AddressSpace types.String `tfsdk:"address_space"`
	Location     types.String `tfsdk:"location"`
	Description  types.String `tfsdk:"description"`
}

func (r *HubResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_hub"
}

func (r *HubResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a hub VNet inside a site.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Hub ID.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"site_id": schema.Int64Attribute{
				Required:    true,
				Description: "ID of the site this hub belongs to. Changing it moves the hub in place, keeping its id and everything under it (IPzilon >= 3.2.0).",
			},
			"name":          schema.StringAttribute{Required: true, Description: "Resource name (must be lowercase — the server normalizes all strings)."},
			"address_space": schema.StringAttribute{Optional: true, Computed: true, Description: "Hub address space CIDR (e.g. 10.0.0.0/16). Maximum size /8.", PlanModifiers: []planmodifier.String{cidrLimits(false)}},
			"location":      schema.StringAttribute{Optional: true, Computed: true, Description: "Free-text location label (e.g. West Europe)."},
			"description":   schema.StringAttribute{Optional: true, Computed: true, Description: "Free-text description."},
		},
	}
}

func (r *HubResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("got %T", req.ProviderData))
		return
	}
	r.client = c
}

func hubFromAPI(h client.Hub) hubModel {
	return hubModel{
		ID:           types.Int64Value(h.ID),
		SiteID:       types.Int64Value(h.SiteID),
		Name:         types.StringValue(h.Name),
		AddressSpace: types.StringPointerValue(h.AddressSpace),
		Location:     types.StringPointerValue(h.Location),
		Description:  types.StringPointerValue(h.Description),
	}
}

func (r *HubResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan hubModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var h client.Hub
	if err := r.client.Post(ctx, "/hubs/", client.HubCreate{
		SiteID:       plan.SiteID.ValueInt64(),
		Name:         plan.Name.ValueString(),
		AddressSpace: strPtr(plan.AddressSpace),
		Location:     strPtr(plan.Location),
		Description:  strPtr(plan.Description),
	}, &h); err != nil {
		resp.Diagnostics.AddError("Create hub failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, hubFromAPI(h))...)
}

func (r *HubResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state hubModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var h client.Hub
	if err := r.client.Get(ctx, fmt.Sprintf("/hubs/%d", state.ID.ValueInt64()), &h); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read hub failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, hubFromAPI(h))...)
}

func (r *HubResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan hubModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	var state hubModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name := plan.Name.ValueString()
	body := client.HubUpdate{
		Name:         &name,
		AddressSpace: strPtr(plan.AddressSpace),
		Location:     strPtr(plan.Location),
		Description:  strPtr(plan.Description),
	}
	move := !plan.SiteID.Equal(state.SiteID)
	if move {
		if !requireHubMove(r.client, &resp.Diagnostics) {
			return
		}
		body.SiteID = plan.SiteID.ValueInt64Pointer()
	}
	var h client.Hub
	if err := r.client.Patch(ctx, fmt.Sprintf("/hubs/%d", state.ID.ValueInt64()), body, &h); err != nil {
		resp.Diagnostics.AddError("Update hub failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, hubFromAPI(h))...)
	// An IPzilon older than 3.2.0 (reporting an unknown version) answers 200
	// without moving the hub; the state keeps the real site.
	if move && h.SiteID != plan.SiteID.ValueInt64() {
		resp.Diagnostics.AddError("IPzilon version not supported", fmt.Sprintf("moving ipzilon_hub to another site requires IPzilon >= %s: the server ignored site_id (hub %d is still in site %d)", client.MinHubMoveAPIVersion, h.ID, h.SiteID))
	}
}

// ModifyPlan reports an IPzilon older than 3.2.0 already in plan when the
// site of an existing hub changes.
func (r *HubResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if r.client == nil || req.Plan.Raw.IsNull() || req.State.Raw.IsNull() {
		return
	}
	var planned, prior types.Int64
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("site_id"), &planned)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("site_id"), &prior)...)
	if planned.IsUnknown() || planned.Equal(prior) {
		return
	}
	requireHubMove(r.client, &resp.Diagnostics)
}

// requireHubMove reports whether the server can move a hub to another site
// (IPzilon >= 3.2.0), adding an error when it cannot. It makes no request.
func requireHubMove(c *client.Client, diags *diag.Diagnostics) bool {
	if err := c.RequireAPIVersion(client.MinHubMoveAPIVersion, "moving ipzilon_hub to another site"); err != nil {
		diags.AddError("IPzilon version not supported", err.Error())
		return false
	}
	return true
}

func (r *HubResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state hubModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, fmt.Sprintf("/hubs/%d", state.ID.ValueInt64())); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete hub failed", err.Error())
	}
}

func (r *HubResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importByID(ctx, req, resp)
}
