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

var _ resource.Resource = &SubnetResource{}
var _ resource.ResourceWithImportState = &SubnetResource{}

type SubnetResource struct{ client *client.Client }

func NewSubnetResource() resource.Resource { return &SubnetResource{} }

type subnetModel struct {
	ID          types.Int64  `tfsdk:"id"`
	NetworkID   types.Int64  `tfsdk:"network_id"`
	Name        types.String `tfsdk:"name"`
	CIDR        types.String `tfsdk:"cidr"`
	Description types.String `tfsdk:"description"`
}

func (r *SubnetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_subnet"
}

func (r *SubnetResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a subnet with an explicit CIDR inside a network. Creating a subnet auto-populates all IP records.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"network_id": schema.Int64Attribute{
				Required:    true,
				Description: "Network this subnet belongs to.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"name":        schema.StringAttribute{Required: true, Description: "Resource name (must be lowercase — the server normalizes all strings)."},
			"cidr":        schema.StringAttribute{Required: true, Description: "Subnet CIDR. Changing this repopulates all IP records."},
			"description": schema.StringAttribute{Optional: true, Computed: true, Description: "Free-text description."},
		},
	}
}

func (r *SubnetResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func subnetFromAPI(s client.Subnet) subnetModel {
	return subnetModel{
		ID:          types.Int64Value(s.ID),
		NetworkID:   types.Int64Value(s.NetworkID),
		Name:        types.StringValue(s.Name),
		CIDR:        types.StringValue(s.CIDR),
		Description: types.StringPointerValue(s.Description),
	}
}

func (r *SubnetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan subnetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var s client.Subnet
	if err := r.client.Post("/subnets/", client.SubnetCreate{
		NetworkID:   plan.NetworkID.ValueInt64(),
		Name:        plan.Name.ValueString(),
		CIDR:        plan.CIDR.ValueString(),
		Description: strPtr(plan.Description),
	}, &s); err != nil {
		resp.Diagnostics.AddError("Create subnet failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, subnetFromAPI(s))...)
}

func (r *SubnetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state subnetModel
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
	resp.Diagnostics.Append(resp.State.Set(ctx, subnetFromAPI(s))...)
}

func (r *SubnetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan subnetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	var state subnetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name := plan.Name.ValueString()
	cidr := plan.CIDR.ValueString()
	var s client.Subnet
	if err := r.client.Patch(fmt.Sprintf("/subnets/%d", state.ID.ValueInt64()), client.SubnetUpdate{
		Name:        &name,
		CIDR:        &cidr,
		Description: strPtr(plan.Description),
	}, &s); err != nil {
		resp.Diagnostics.AddError("Update subnet failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, subnetFromAPI(s))...)
}

func (r *SubnetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state subnetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(fmt.Sprintf("/subnets/%d", state.ID.ValueInt64())); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete subnet failed", err.Error())
	}
}

func (r *SubnetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importByID(ctx, req, resp)
}
