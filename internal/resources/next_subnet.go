package resources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/mdepedrof/terraform-provider-ipzilon/internal/client"
)

var _ resource.Resource = &NextSubnetResource{}
var _ resource.ResourceWithImportState = &NextSubnetResource{}
var _ resource.ResourceWithModifyPlan = &NextSubnetResource{}

type NextSubnetResource struct{ client *client.Client }

func NewNextSubnetResource() resource.Resource { return &NextSubnetResource{} }

type nextSubnetModel struct {
	ID           types.Int64  `tfsdk:"id"`
	NetworkID    types.Int64  `tfsdk:"network_id"`
	PrefixLength types.Int64  `tfsdk:"prefix_length"`
	Name         types.String `tfsdk:"name"`
	Description  types.String `tfsdk:"description"`
	CIDR         types.String `tfsdk:"cidr"`
	ZoneID       types.Int64  `tfsdk:"zone_id"`
}

// nextSubnetFromAPI builds the whole state from the API object; it is the only
// place that fills nextSubnetModel (Create, Read and Update). prefix_length is
// derived from the CIDR the API returns.
func nextSubnetFromAPI(sub client.Subnet) (nextSubnetModel, error) {
	prefixLength, err := prefixLengthValue(sub.CIDR)
	if err != nil {
		return nextSubnetModel{}, err
	}
	return nextSubnetModel{
		ID:           types.Int64Value(sub.ID),
		NetworkID:    types.Int64Value(sub.NetworkID),
		PrefixLength: prefixLength,
		Name:         types.StringValue(sub.Name),
		Description:  types.StringPointerValue(sub.Description),
		CIDR:         types.StringValue(sub.CIDR),
		ZoneID:       types.Int64PointerValue(sub.ZoneID),
	}, nil
}

func setNextSubnetState(ctx context.Context, sub client.Subnet, state *tfsdk.State, diags *diag.Diagnostics) {
	model, err := nextSubnetFromAPI(sub)
	if err != nil {
		diags.AddError("Invalid subnet returned by the API", err.Error())
		return
	}
	diags.Append(state.Set(ctx, model)...)
}

func (r *NextSubnetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_next_subnet"
}

func (r *NextSubnetResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Atomically reserves the first available subnet block of a given prefix length inside a network. The CIDR is assigned by the server.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"network_id": schema.Int64Attribute{
				Required:    true,
				Description: "Network to allocate from.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"prefix_length": schema.Int64Attribute{
				Required:    true,
				Description: "Desired prefix length (e.g. 27 for /27).",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"name":        schema.StringAttribute{Optional: true, Computed: true, Description: "Resource name."},
			"description": schema.StringAttribute{Optional: true, Computed: true, Description: "Free-text description."},
			"cidr": schema.StringAttribute{
				Computed:    true,
				Description: "Assigned CIDR block (computed by server).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"zone_id": zoneIDAttribute(
				"Zone to allocate the block from (IPzilon >= 3.1.0). Without it, in a network that has zones IPzilon allocates outside every zone. Always reflects the zone computed by IPzilon (null if none); changing it never replaces the subnet.",
				int64planmodifier.UseStateForUnknown(),
			),
		},
	}
}

func (r *NextSubnetResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *NextSubnetResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	modifyPlanZones(ctx, r.client, "zone_id in ipzilon_next_subnet", req, resp)
}

func (r *NextSubnetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan nextSubnetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	zoneID := configZoneID(ctx, req.Config, &resp.Diagnostics)
	if zoneID != nil && !requireZones(r.client, "zone_id in ipzilon_next_subnet", &resp.Diagnostics) {
		return
	}
	var s client.Subnet
	if err := r.client.Post(ctx,
		fmt.Sprintf("/networks/%d/next-available-subnet", plan.NetworkID.ValueInt64()),
		client.AllocateSubnetBody{
			PrefixLength: plan.PrefixLength.ValueInt64(),
			Name:         strPtr(plan.Name),
			Description:  strPtr(plan.Description),
			ZoneID:       zoneID,
		}, &s,
	); err != nil {
		if summary, detail, ok := allocationErrorDiag(err, plan.PrefixLength.ValueInt64(), "ipzilon_subnet"); ok {
			resp.Diagnostics.AddError(summary, detail)
			return
		}
		resp.Diagnostics.AddError("Reserve next subnet failed", err.Error())
		return
	}
	if !checkZoneApplied(ctx, r.client, "zone_id in ipzilon_next_subnet", zoneID, s, true, &resp.Diagnostics) {
		return
	}
	setNextSubnetState(ctx, s, &resp.State, &resp.Diagnostics)
}

func (r *NextSubnetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state nextSubnetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var s client.Subnet
	if err := r.client.Get(ctx, fmt.Sprintf("/subnets/%d", state.ID.ValueInt64()), &s); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read subnet failed", err.Error())
		return
	}
	setNextSubnetState(ctx, s, &resp.State, &resp.Diagnostics)
}

func (r *NextSubnetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan nextSubnetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	var state nextSubnetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	zoneID := configZoneID(ctx, req.Config, &resp.Diagnostics)
	if zoneID != nil && !requireZones(r.client, "zone_id in ipzilon_next_subnet", &resp.Diagnostics) {
		return
	}
	name := plan.Name.ValueString()
	var s client.Subnet
	if err := r.client.Patch(ctx, fmt.Sprintf("/subnets/%d", state.ID.ValueInt64()), client.SubnetUpdate{
		Name:        &name,
		Description: strPtr(plan.Description),
		ZoneID:      zoneID,
	}, &s); err != nil {
		resp.Diagnostics.AddError("Update subnet failed", err.Error())
		return
	}
	if !checkZoneApplied(ctx, r.client, "zone_id in ipzilon_next_subnet", zoneID, s, false, &resp.Diagnostics) {
		return
	}
	setNextSubnetState(ctx, s, &resp.State, &resp.Diagnostics)
}

func (r *NextSubnetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state nextSubnetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, fmt.Sprintf("/subnets/%d", state.ID.ValueInt64())); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete subnet failed", err.Error())
	}
}

func (r *NextSubnetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importByID(ctx, req, resp)
}
