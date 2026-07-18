package resources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/mdepedrof/terraform-provider-ipzilon/internal/client"
)

var _ resource.Resource = &LandingZoneResource{}
var _ resource.ResourceWithImportState = &LandingZoneResource{}

type LandingZoneResource struct{ client *client.Client }

func NewLandingZoneResource() resource.Resource { return &LandingZoneResource{} }

type landingZoneModel struct {
	ID          types.Int64  `tfsdk:"id"`
	HubID       types.Int64  `tfsdk:"hub_id"`
	ParentID    types.Int64  `tfsdk:"parent_id"`
	Name        types.String `tfsdk:"name"`
	CIDR        types.String `tfsdk:"cidr"`
	Description types.String `tfsdk:"description"`
}

func (r *LandingZoneResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_landing_zone"
}

func (r *LandingZoneResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a landing zone inside a hub. Supports parent/child hierarchy (max 2 levels).",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"hub_id": schema.Int64Attribute{
				Required:    true,
				Description: "Hub this landing zone belongs to.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"parent_id":   schema.Int64Attribute{Optional: true, Computed: true, Description: "Parent landing zone ID (omit for root-level LZ)."},
			"name":        schema.StringAttribute{Required: true},
			"cidr":        schema.StringAttribute{Optional: true, Computed: true, Description: "Optional CIDR assigned to this landing zone."},
			"description": schema.StringAttribute{Optional: true, Computed: true},
		},
	}
}

func (r *LandingZoneResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func lzFromAPI(lz client.LandingZone) landingZoneModel {
	return landingZoneModel{
		ID:          types.Int64Value(lz.ID),
		HubID:       types.Int64Value(lz.HubID),
		ParentID:    types.Int64PointerValue(lz.ParentID),
		Name:        types.StringValue(lz.Name),
		CIDR:        types.StringPointerValue(lz.CIDR),
		Description: types.StringPointerValue(lz.Description),
	}
}

func (r *LandingZoneResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan landingZoneModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var lz client.LandingZone
	if err := r.client.Post("/landing-zones/", client.LandingZoneCreate{
		HubID:       plan.HubID.ValueInt64(),
		ParentID:    int64Ptr(plan.ParentID),
		Name:        plan.Name.ValueString(),
		CIDR:        strPtr(plan.CIDR),
		Description: strPtr(plan.Description),
	}, &lz); err != nil {
		resp.Diagnostics.AddError("Create landing zone failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, lzFromAPI(lz))...)
}

func (r *LandingZoneResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state landingZoneModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var lz client.LandingZone
	if err := r.client.Get(fmt.Sprintf("/landing-zones/%d", state.ID.ValueInt64()), &lz); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read landing zone failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, lzFromAPI(lz))...)
}

func (r *LandingZoneResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan landingZoneModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	var state landingZoneModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name := plan.Name.ValueString()
	var lz client.LandingZone
	if err := r.client.Patch(fmt.Sprintf("/landing-zones/%d", state.ID.ValueInt64()), client.LandingZoneUpdate{
		ParentID:    int64Ptr(plan.ParentID),
		Name:        &name,
		CIDR:        strPtr(plan.CIDR),
		Description: strPtr(plan.Description),
	}, &lz); err != nil {
		resp.Diagnostics.AddError("Update landing zone failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, lzFromAPI(lz))...)
}

func (r *LandingZoneResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state landingZoneModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(fmt.Sprintf("/landing-zones/%d", state.ID.ValueInt64())); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete landing zone failed", err.Error())
	}
}

func (r *LandingZoneResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importByID(ctx, req, resp)
}
