package datasources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/mdepedrof/terraform-provider-ipzilon/internal/client"
)

var _ datasource.DataSource = &ScopesDataSource{}

type ScopesDataSource struct{ client *client.Client }

func NewScopesDataSource() datasource.DataSource { return &ScopesDataSource{} }

type scopesModel struct {
	ID       types.Int64  `tfsdk:"id"`
	HubID    types.Int64  `tfsdk:"hub_id"`
	ParentID types.Int64  `tfsdk:"parent_id"`
	RootOnly types.Bool   `tfsdk:"root_only"`
	Kind     types.String `tfsdk:"kind"`
	Items    []scopeItem  `tfsdk:"items"`
}

type scopeItem struct {
	ID          types.Int64  `tfsdk:"id"`
	HubID       types.Int64  `tfsdk:"hub_id"`
	ParentID    types.Int64  `tfsdk:"parent_id"`
	Name        types.String `tfsdk:"name"`
	Kind        types.String `tfsdk:"kind"`
	CIDR        types.String `tfsdk:"cidr"`
	Description types.String `tfsdk:"description"`
}

var scopeItemSchema = schema.NestedAttributeObject{
	Attributes: map[string]schema.Attribute{
		"id":          schema.Int64Attribute{Computed: true},
		"hub_id":      schema.Int64Attribute{Computed: true},
		"parent_id":   schema.Int64Attribute{Computed: true},
		"name":        schema.StringAttribute{Computed: true, Description: "Scope name."},
		"kind":        schema.StringAttribute{Computed: true, Description: "Scope kind (landing_zone or project)."},
		"cidr":        schema.StringAttribute{Computed: true, Description: "CIDR block assigned to this scope."},
		"description": schema.StringAttribute{Computed: true, Description: "Free-text description."},
	},
}

func (d *ScopesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_scopes"
}

func (d *ScopesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List scopes (landing zones and projects). Provide id (singular) OR hub_id with optional parent_id/root_only/kind filter.",
		Attributes: map[string]schema.Attribute{
			"id":        schema.Int64Attribute{Optional: true, Description: "Lookup a single scope by ID."},
			"hub_id":    schema.Int64Attribute{Optional: true, Description: "List scopes for this hub."},
			"parent_id": schema.Int64Attribute{Optional: true, Description: "Filter by parent scope ID."},
			"root_only": schema.BoolAttribute{Optional: true, Description: "When true, return only root-level scopes (parent_id IS NULL)."},
			"kind": schema.StringAttribute{
				Optional:    true,
				Description: "Optional client-side filter: only return items with this kind (landing_zone or project). Not supported server-side.",
				Validators:  []validator.String{stringvalidator.OneOf("landing_zone", "project")},
			},
			"items": schema.ListNestedAttribute{Computed: true, NestedObject: scopeItemSchema},
		},
	}
}

func (d *ScopesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureClient(req, resp)
}

func scopeToItem(s client.Scope) scopeItem {
	return scopeItem{
		ID:          types.Int64Value(s.ID),
		HubID:       types.Int64Value(s.HubID),
		ParentID:    types.Int64PointerValue(s.ParentID),
		Name:        types.StringValue(s.Name),
		Kind:        types.StringValue(s.Kind),
		CIDR:        types.StringPointerValue(s.CIDR),
		Description: types.StringPointerValue(s.Description),
	}
}

func (d *ScopesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg scopesModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hasID := !cfg.ID.IsNull() && !cfg.ID.IsUnknown()
	hasHub := !cfg.HubID.IsNull() && !cfg.HubID.IsUnknown()

	if !validateFilters(ctx, hasID, hasHub, resp) {
		return
	}

	var items []scopeItem
	if hasID {
		var s client.Scope
		if err := d.client.Get(fmt.Sprintf("/scopes/%d", cfg.ID.ValueInt64()), &s); err != nil {
			resp.Diagnostics.AddError("Get scope failed", err.Error())
			return
		}
		items = []scopeItem{scopeToItem(s)}
	} else {
		url := fmt.Sprintf("/hubs/%d/scopes", cfg.HubID.ValueInt64())
		hasParent := !cfg.ParentID.IsNull() && !cfg.ParentID.IsUnknown()
		rootOnly := !cfg.RootOnly.IsNull() && cfg.RootOnly.ValueBool()

		if hasParent && rootOnly {
			resp.Diagnostics.AddError("Conflicting filters", "parent_id and root_only are mutually exclusive.")
			return
		}
		if hasParent {
			url += fmt.Sprintf("?parent_id=%d", cfg.ParentID.ValueInt64())
		} else if rootOnly {
			url += "?root_only=true"
		}

		var scopes []client.Scope
		if err := d.client.Get(url, &scopes); err != nil {
			resp.Diagnostics.AddError("List scopes failed", err.Error())
			return
		}
		for _, s := range scopes {
			items = append(items, scopeToItem(s))
		}
	}

	if hasKind := !cfg.Kind.IsNull() && !cfg.Kind.IsUnknown(); hasKind {
		filtered := items[:0]
		for _, it := range items {
			if it.Kind.ValueString() == cfg.Kind.ValueString() {
				filtered = append(filtered, it)
			}
		}
		items = filtered
	}

	cfg.Items = items
	resp.Diagnostics.Append(resp.State.Set(ctx, cfg)...)
}
