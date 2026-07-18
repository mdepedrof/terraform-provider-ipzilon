package resources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/mdepedrof/terraform-provider-ipzilon/internal/client"
)

var _ resource.Resource = &LastSubnetResource{}
var _ resource.ResourceWithImportState = &LastSubnetResource{}

type LastSubnetResource struct{ client *client.Client }

func NewLastSubnetResource() resource.Resource { return &LastSubnetResource{} }

type lastSubnetModel struct {
	ID           types.Int64  `tfsdk:"id"`
	NetworkID    types.Int64  `tfsdk:"network_id"`
	PrefixLength types.Int64  `tfsdk:"prefix_length"`
	Name         types.String `tfsdk:"name"`
	Description  types.String `tfsdk:"description"`
	CIDR         types.String `tfsdk:"cidr"`
}

func (r *LastSubnetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_last_subnet"
}

func (r *LastSubnetResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Atomically reserves the last available subnet block of a given prefix length inside a network. The CIDR is assigned by the server.",
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
		},
	}
}

func (r *LastSubnetResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *LastSubnetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan lastSubnetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var s client.Subnet
	if err := r.client.Post(
		fmt.Sprintf("/networks/%d/last-available-subnet", plan.NetworkID.ValueInt64()),
		client.AllocateSubnetBody{
			PrefixLength: plan.PrefixLength.ValueInt64(),
			Name:         strPtr(plan.Name),
			Description:  strPtr(plan.Description),
		}, &s,
	); err != nil {
		resp.Diagnostics.AddError("Reserve last subnet failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, lastSubnetModel{
		ID:           types.Int64Value(s.ID),
		NetworkID:    types.Int64Value(s.NetworkID),
		PrefixLength: plan.PrefixLength,
		Name:         types.StringValue(s.Name),
		Description:  types.StringPointerValue(s.Description),
		CIDR:         types.StringValue(s.CIDR),
	})...)
}

func (r *LastSubnetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state lastSubnetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var s client.Subnet
	if err := r.client.Get(fmt.Sprintf("/subnets/%d", state.ID.ValueInt64()), &s); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read subnet failed", err.Error())
		return
	}
	state.Name = types.StringValue(s.Name)
	state.Description = types.StringPointerValue(s.Description)
	state.CIDR = types.StringValue(s.CIDR)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *LastSubnetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan lastSubnetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	var state lastSubnetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name := plan.Name.ValueString()
	var s client.Subnet
	if err := r.client.Patch(fmt.Sprintf("/subnets/%d", state.ID.ValueInt64()), client.SubnetUpdate{
		Name:        &name,
		Description: strPtr(plan.Description),
	}, &s); err != nil {
		resp.Diagnostics.AddError("Update subnet failed", err.Error())
		return
	}
	state.Name = types.StringValue(s.Name)
	state.Description = types.StringPointerValue(s.Description)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *LastSubnetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state lastSubnetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(fmt.Sprintf("/subnets/%d", state.ID.ValueInt64())); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete subnet failed", err.Error())
	}
}

func (r *LastSubnetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importByID(ctx, req, resp)
}
