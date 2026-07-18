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

var _ resource.Resource = &NetworkResource{}
var _ resource.ResourceWithImportState = &NetworkResource{}

type NetworkResource struct{ client *client.Client }

func NewNetworkResource() resource.Resource { return &NetworkResource{} }

type networkModel struct {
	ID            types.Int64  `tfsdk:"id"`
	LandingZoneID types.Int64  `tfsdk:"landing_zone_id"`
	Name          types.String `tfsdk:"name"`
	CIDR          types.String `tfsdk:"cidr"`
	Description   types.String `tfsdk:"description"`
}

func (r *NetworkResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_network"
}

func (r *NetworkResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a VNet/spoke network inside a landing zone.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"landing_zone_id": schema.Int64Attribute{
				Required:    true,
				Description: "Landing zone this network belongs to.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"name":            schema.StringAttribute{Required: true, Description: "Resource name (must be lowercase — the server normalizes all strings)."},
			"cidr":            schema.StringAttribute{Required: true, Description: "Network CIDR (e.g. 10.0.1.0/24)."},
			"description":     schema.StringAttribute{Optional: true, Computed: true, Description: "Free-text description."},
		},
	}
}

func (r *NetworkResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func networkFromAPI(n client.Network) networkModel {
	return networkModel{
		ID:            types.Int64Value(n.ID),
		LandingZoneID: types.Int64Value(n.LandingZoneID),
		Name:          types.StringValue(n.Name),
		CIDR:          types.StringValue(n.CIDR),
		Description:   types.StringPointerValue(n.Description),
	}
}

func (r *NetworkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan networkModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var n client.Network
	if err := r.client.Post("/networks/", client.NetworkCreate{
		LandingZoneID: plan.LandingZoneID.ValueInt64(),
		Name:          plan.Name.ValueString(),
		CIDR:          plan.CIDR.ValueString(),
		Description:   strPtr(plan.Description),
	}, &n); err != nil {
		resp.Diagnostics.AddError("Create network failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, networkFromAPI(n))...)
}

func (r *NetworkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state networkModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var n client.Network
	if err := r.client.Get(fmt.Sprintf("/networks/%d", state.ID.ValueInt64()), &n); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read network failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, networkFromAPI(n))...)
}

func (r *NetworkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan networkModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	var state networkModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	lzID := plan.LandingZoneID.ValueInt64()
	name := plan.Name.ValueString()
	cidr := plan.CIDR.ValueString()
	var n client.Network
	if err := r.client.Patch(fmt.Sprintf("/networks/%d", state.ID.ValueInt64()), client.NetworkUpdate{
		LandingZoneID: &lzID,
		Name:          &name,
		CIDR:          &cidr,
		Description:   strPtr(plan.Description),
	}, &n); err != nil {
		resp.Diagnostics.AddError("Update network failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, networkFromAPI(n))...)
}

func (r *NetworkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state networkModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(fmt.Sprintf("/networks/%d", state.ID.ValueInt64())); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete network failed", err.Error())
	}
}

func (r *NetworkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importByID(ctx, req, resp)
}
