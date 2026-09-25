package datasources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/mdepedrof/terraform-provider-ipzilon/internal/client"
)

var _ datasource.DataSource = &SitesDataSource{}

type SitesDataSource struct{ client *client.Client }

func NewSitesDataSource() datasource.DataSource { return &SitesDataSource{} }

type sitesModel struct {
	ID    types.Int64  `tfsdk:"id"`
	Name  types.String `tfsdk:"name"`
	Items []siteItem   `tfsdk:"items"`
}

type siteItem struct {
	ID          types.Int64  `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Type        types.String `tfsdk:"type"`
	Description types.String `tfsdk:"description"`
}

var siteItemSchema = schema.NestedAttributeObject{
	Attributes: map[string]schema.Attribute{
		"id":          schema.Int64Attribute{Computed: true},
		"name":        schema.StringAttribute{Computed: true, Description: "Site name."},
		"type":        schema.StringAttribute{Computed: true, Description: "Site type."},
		"description": schema.StringAttribute{Computed: true, Description: "Free-text description."},
	},
}

func (d *SitesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sites"
}

func (d *SitesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List sites. Provide id (singular lookup), name (server-side filter) or neither (list all sites).",
		Attributes: map[string]schema.Attribute{
			"id":    schema.Int64Attribute{Optional: true, Description: "Lookup a single site by ID."},
			"name":  schema.StringAttribute{Optional: true, Description: "Filter sites by exact name match."},
			"items": schema.ListNestedAttribute{Computed: true, NestedObject: siteItemSchema},
		},
	}
}

func (d *SitesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureClient(req, resp)
}

func siteToItem(s client.Site) siteItem {
	return siteItem{
		ID:          types.Int64Value(s.ID),
		Name:        types.StringValue(s.Name),
		Type:        types.StringValue(s.Type),
		Description: types.StringPointerValue(s.Description),
	}
}

func (d *SitesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg sitesModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hasID := !cfg.ID.IsNull() && !cfg.ID.IsUnknown()
	hasName := !cfg.Name.IsNull() && !cfg.Name.IsUnknown()

	if hasID && hasName {
		resp.Diagnostics.AddError("Conflicting filters", "Provide either id or name, not both.")
		return
	}

	var items []siteItem
	if hasID {
		var s client.Site
		if err := d.client.Get(fmt.Sprintf("/sites/%d", cfg.ID.ValueInt64()), &s); err != nil {
			resp.Diagnostics.AddError("Get site failed", err.Error())
			return
		}
		items = []siteItem{siteToItem(s)}
	} else {
		var sites []client.Site
		if err := d.client.Get(sitesURL(stringFilter(cfg.Name)), &sites); err != nil {
			resp.Diagnostics.AddError("List sites failed", err.Error())
			return
		}
		for _, s := range sites {
			items = append(items, siteToItem(s))
		}
	}

	cfg.Items = items
	resp.Diagnostics.Append(resp.State.Set(ctx, cfg)...)
}
