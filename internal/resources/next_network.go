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

var _ resource.Resource = &NextNetworkResource{}
var _ resource.ResourceWithImportState = &NextNetworkResource{}

type NextNetworkResource struct{ client *client.Client }

func NewNextNetworkResource() resource.Resource { return &NextNetworkResource{} }

type nextNetworkModel struct {
	ID           types.Int64  `tfsdk:"id"`
	ScopeID      types.Int64  `tfsdk:"scope_id"`
	PrefixLength types.Int64  `tfsdk:"prefix_length"`
	Name         types.String `tfsdk:"name"`
	Description  types.String `tfsdk:"description"`
	CIDR         types.String `tfsdk:"cidr"`
}

func (r *NextNetworkResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_next_network"
}

func (r *NextNetworkResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Atomically reserves the first available network block of a given prefix length inside a scope. The CIDR is assigned by the server.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"scope_id": schema.Int64Attribute{
				Required:    true,
				Description: "Scope to allocate from.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"prefix_length": schema.Int64Attribute{
				Required:    true,
				Description: "Desired prefix length (e.g. 24 for /24).",
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

func (r *NextNetworkResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *NextNetworkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan nextNetworkModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var n client.Network
	if err := r.client.Post(
		fmt.Sprintf("/scopes/%d/next-available-network", plan.ScopeID.ValueInt64()),
		client.AllocateNetworkBody{
			PrefixLength: plan.PrefixLength.ValueInt64(),
			Name:         strPtr(plan.Name),
			Description:  strPtr(plan.Description),
		}, &n,
	); err != nil {
		resp.Diagnostics.AddError("Reserve next network failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, nextNetworkModel{
		ID:           types.Int64Value(n.ID),
		ScopeID:      types.Int64Value(n.ScopeID),
		PrefixLength: plan.PrefixLength,
		Name:         types.StringValue(n.Name),
		Description:  types.StringPointerValue(n.Description),
		CIDR:         types.StringValue(n.CIDR),
	})...)
}

func (r *NextNetworkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state nextNetworkModel
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
	state.Name = types.StringValue(n.Name)
	state.Description = types.StringPointerValue(n.Description)
	state.CIDR = types.StringValue(n.CIDR)
	state.ScopeID = types.Int64Value(n.ScopeID)
	if prefixLength, err := cidrPrefixLength(n.CIDR); err == nil {
		state.PrefixLength = types.Int64Value(prefixLength)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *NextNetworkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan nextNetworkModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	var state nextNetworkModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name := plan.Name.ValueString()
	var n client.Network
	if err := r.client.Patch(fmt.Sprintf("/networks/%d", state.ID.ValueInt64()), client.NetworkUpdate{
		Name:        &name,
		Description: strPtr(plan.Description),
	}, &n); err != nil {
		resp.Diagnostics.AddError("Update network failed", err.Error())
		return
	}
	state.Name = types.StringValue(n.Name)
	state.Description = types.StringPointerValue(n.Description)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *NextNetworkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state nextNetworkModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(fmt.Sprintf("/networks/%d", state.ID.ValueInt64())); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete network failed", err.Error())
	}
}

func (r *NextNetworkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importByID(ctx, req, resp)
}
