package resources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/mdepedrof/terraform-provider-ipzilon/internal/client"
)

var _ resource.Resource = &ScopeResource{}
var _ resource.ResourceWithImportState = &ScopeResource{}

type ScopeResource struct{ client *client.Client }

func NewScopeResource() resource.Resource { return &ScopeResource{} }

type scopeModel struct {
	ID          types.Int64  `tfsdk:"id"`
	HubID       types.Int64  `tfsdk:"hub_id"`
	ParentID    types.Int64  `tfsdk:"parent_id"`
	Name        types.String `tfsdk:"name"`
	Kind        types.String `tfsdk:"kind"`
	CIDR        types.String `tfsdk:"cidr"`
	Description types.String `tfsdk:"description"`
}

func (r *ScopeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_scope"
}

func (r *ScopeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a scope (landing zone or project) inside a hub. Supports parent/child hierarchy (max 4 levels). A root-level scope must have kind = \"landing_zone\"; once a branch enters kind = \"project\", every descendant must also be kind = \"project\".",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"hub_id": schema.Int64Attribute{
				Required:    true,
				Description: "Hub this scope belongs to.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"parent_id": schema.Int64Attribute{Optional: true, Computed: true, Description: "Parent scope ID (omit for root-level scope)."},
			"name":      schema.StringAttribute{Required: true, Description: "Resource name (must be lowercase — the server normalizes all strings)."},
			"kind": schema.StringAttribute{
				Required:    true,
				Description: "Scope kind: \"landing_zone\" or \"project\". Root scopes must be \"landing_zone\"; a \"project\" cannot have a \"landing_zone\" nested under it.",
				Validators:  []validator.String{stringvalidator.OneOf("landing_zone", "project")},
			},
			"cidr":        schema.StringAttribute{Optional: true, Computed: true, Description: "Optional CIDR assigned to this scope.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"description": schema.StringAttribute{Optional: true, Computed: true, Description: "Free-text description."},
		},
	}
}

func (r *ScopeResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func scopeFromAPI(s client.Scope) scopeModel {
	return scopeModel{
		ID:          types.Int64Value(s.ID),
		HubID:       types.Int64Value(s.HubID),
		ParentID:    types.Int64PointerValue(s.ParentID),
		Name:        types.StringValue(s.Name),
		Kind:        types.StringValue(s.Kind),
		CIDR:        types.StringPointerValue(s.CIDR),
		Description: types.StringPointerValue(s.Description),
	}
}

func (r *ScopeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan scopeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var s client.Scope
	if err := r.client.Post("/scopes/", client.ScopeCreate{
		HubID:       plan.HubID.ValueInt64(),
		ParentID:    int64Ptr(plan.ParentID),
		Name:        plan.Name.ValueString(),
		Kind:        plan.Kind.ValueString(),
		CIDR:        strPtr(plan.CIDR),
		Description: strPtr(plan.Description),
	}, &s); err != nil {
		resp.Diagnostics.AddError("Create scope failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, scopeFromAPI(s))...)
}

func (r *ScopeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state scopeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var s client.Scope
	if err := r.client.Get(fmt.Sprintf("/scopes/%d", state.ID.ValueInt64()), &s); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read scope failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, scopeFromAPI(s))...)
}

func (r *ScopeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan scopeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	var state scopeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name := plan.Name.ValueString()
	kind := plan.Kind.ValueString()
	var s client.Scope
	if err := r.client.Patch(fmt.Sprintf("/scopes/%d", state.ID.ValueInt64()), client.ScopeUpdate{
		ParentID:    int64Ptr(plan.ParentID),
		Name:        &name,
		Kind:        &kind,
		CIDR:        strPtr(plan.CIDR),
		Description: strPtr(plan.Description),
	}, &s); err != nil {
		resp.Diagnostics.AddError("Update scope failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, scopeFromAPI(s))...)
}

func (r *ScopeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state scopeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(fmt.Sprintf("/scopes/%d", state.ID.ValueInt64())); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete scope failed", err.Error())
	}
}

func (r *ScopeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importByID(ctx, req, resp)
}
