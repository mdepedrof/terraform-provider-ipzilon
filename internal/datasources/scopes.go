package datasources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/mdepedrof/terraform-provider-ipzilon/internal/client"
)

var (
	_ datasource.DataSource                   = &ScopesDataSource{}
	_ datasource.DataSourceWithValidateConfig = &ScopesDataSource{}
)

type ScopesDataSource struct{ client *client.Client }

func NewScopesDataSource() datasource.DataSource { return &ScopesDataSource{} }

type scopesModel struct {
	ID       types.Int64  `tfsdk:"id"`
	HubID    types.Int64  `tfsdk:"hub_id"`
	ParentID types.Int64  `tfsdk:"parent_id"`
	RootOnly types.Bool   `tfsdk:"root_only"`
	Kind     types.String `tfsdk:"kind"`
	CIDR     types.String `tfsdk:"cidr"`
	Name     types.String `tfsdk:"name"`
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
		"id":          schema.Int64Attribute{Computed: true, Description: "Scope ID."},
		"hub_id":      schema.Int64Attribute{Computed: true, Description: "ID of the hub the scope belongs to."},
		"parent_id":   schema.Int64Attribute{Computed: true, Description: "Parent scope ID (null for root scopes)."},
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
		Description: "List scopes (landing zones and projects). Provide id (singular) or any combination of hub_id, parent_id, root_only, kind, cidr and name. Without hub_id the lookup is global (IPzilon >= 3.2.0).",
		Attributes: map[string]schema.Attribute{
			"id":        schema.Int64Attribute{Optional: true, Description: "Lookup a single scope by ID. Cannot be combined with hub_id."},
			"hub_id":    schema.Int64Attribute{Optional: true, Description: "Filter: scopes of this hub. Optional: without it the lookup is global (IPzilon >= 3.2.0)."},
			"parent_id": schema.Int64Attribute{Optional: true, Description: "Filter: direct children of this parent scope."},
			"root_only": schema.BoolAttribute{Optional: true, Description: "When true, return only root-level scopes (parent_id IS NULL). Requires hub_id."},
			"kind": schema.StringAttribute{
				Optional:    true,
				Description: "Filter: only return scopes of this kind (landing_zone or project). With hub_id it is filtered client-side; without hub_id it is filtered server-side (IPzilon >= 3.2.0).",
				Validators:  []validator.String{stringvalidator.OneOf("landing_zone", "project")},
			},
			"cidr":  schema.StringAttribute{Optional: true, Description: "Filter: cidr match (server-side). With hub_id it is an exact text match; without hub_id it compares the network, so the value must be a CIDR without host bits."},
			"name":  schema.StringAttribute{Optional: true, Description: "Filter: exact name match (server-side)."},
			"items": schema.ListNestedAttribute{Computed: true, NestedObject: scopeItemSchema, Description: "Matching scopes; an empty list when nothing matches."},
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

	if hasID && hasHub {
		resp.Diagnostics.AddError("Conflicting filters", "Provide either id or hub_id, not both.")
		return
	}

	items := []scopeItem{}
	if hasID {
		var s client.Scope
		if err := d.client.Get(ctx, fmt.Sprintf("/scopes/%d", cfg.ID.ValueInt64()), &s); err != nil {
			resp.Diagnostics.AddError("Get scope failed", err.Error())
			return
		}
		items = []scopeItem{scopeToItem(s)}
	} else if hasHub {
		hasParent := !cfg.ParentID.IsNull() && !cfg.ParentID.IsUnknown()
		rootOnly := !cfg.RootOnly.IsNull() && cfg.RootOnly.ValueBool()

		if hasParent && rootOnly {
			resp.Diagnostics.AddError("Conflicting filters", "parent_id and root_only are mutually exclusive.")
			return
		}

		reqURL := hubScopesURL(
			cfg.HubID.ValueInt64(),
			hasParent, cfg.ParentID.ValueInt64(),
			rootOnly,
			stringFilter(cfg.CIDR),
			stringFilter(cfg.Name),
		)

		scopes, err := client.GetAll[client.Scope](ctx, d.client, reqURL)
		if err != nil {
			resp.Diagnostics.AddError("List scopes failed", err.Error())
			return
		}
		for _, s := range scopes {
			items = append(items, scopeToItem(s))
		}
	} else {
		const feature = "ipzilon_scopes without hub_id"
		if !requireGlobalLists(d.client, feature, &resp.Diagnostics) {
			return
		}
		reqURL := globalScopesURL(stringFilter(cfg.Name), stringFilter(cfg.CIDR), stringFilter(cfg.Kind), int64Filter(cfg.ParentID))
		scopes, err := client.GetAll[client.Scope](ctx, d.client, reqURL)
		if err != nil {
			globalListError(&resp.Diagnostics, "List scopes failed", feature, err)
			return
		}
		for _, s := range scopes {
			items = append(items, scopeToItem(s))
		}
	}

	// The listing of a hub has no kind filter, so kind is applied here; the
	// global listing already filtered it in the server.
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

// ValidateConfig checks in plan the filters the global listing does not
// accept (root_only without hub_id) and that cidr is a network address when
// the lookup is global (no id and no hub_id).
func (d *ScopesDataSource) ValidateConfig(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	var cfg scopesModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if cfg.RootOnly.ValueBool() && cfg.HubID.IsNull() {
		resp.Diagnostics.AddAttributeError(path.Root("root_only"), "Missing filter", "root_only requires hub_id: the global scope listing has no root_only filter")
	}
	if cfg.ID.IsNull() && cfg.HubID.IsNull() {
		validateGlobalCIDR(ctx, req.Config, "cidr", &resp.Diagnostics)
	}
}
