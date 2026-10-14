package datasources

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestItemsAttribute(t *testing.T) {
	fields := []itemField{
		{"id", types.Int64Type, "Hub ID."},
		{"name", types.StringType, "Hub name."},
		{"is_azure_reserved", types.BoolType, "True for IPs auto-reserved by Azure (.1/.2/.3/broadcast)."},
	}
	got := itemsAttribute("Matching hubs; an empty list when nothing matches.", fields)

	if !got.Computed {
		t.Error("items is not computed")
	}
	want := types.ObjectType{AttrTypes: map[string]attr.Type{
		"id":                types.Int64Type,
		"name":              types.StringType,
		"is_azure_reserved": types.BoolType,
	}}
	if !got.ElementType.Equal(want) {
		t.Errorf("ElementType = %v, want %v", got.ElementType, want)
	}
	const wantDesc = "Matching hubs; an empty list when nothing matches. Each item has: " +
		"`id` — Hub ID. `name` — Hub name. " +
		"`is_azure_reserved` — True for IPs auto-reserved by Azure (.1/.2/.3/broadcast)."
	if got.Description != wantDesc {
		t.Errorf("Description = %q, want %q", got.Description, wantDesc)
	}
}

// TestListDataSourcesItemsAreMockable guards issue #26: items must stay a
// plain list of objects (terraform test cannot mock nested attributes) with
// the same element type as before, so state and expressions do not change.
func TestListDataSourcesItemsAreMockable(t *testing.T) {
	var (
		i64 = types.Int64Type
		str = types.StringType
	)
	cases := []struct {
		name   string
		new    func() datasource.DataSource
		fields map[string]attr.Type
	}{
		{"hubs", NewHubsDataSource, map[string]attr.Type{
			"id": i64, "site_id": i64, "name": str, "address_space": str, "location": str, "description": str,
		}},
		{"ip_addresses", NewIPAddressesDataSource, map[string]attr.Type{
			"id": i64, "subnet_id": i64, "address": str, "status": str, "is_azure_reserved": types.BoolType,
			"hostname": str, "description": str,
		}},
		{"network_zones", NewNetworkZonesDataSource, map[string]attr.Type{
			"id": i64, "network_id": i64, "name": str, "cidr": str, "description": str,
		}},
		{"networks", NewNetworksDataSource, map[string]attr.Type{
			"id": i64, "scope_id": i64, "name": str, "cidr": str, "description": str,
		}},
		{"scopes", NewScopesDataSource, map[string]attr.Type{
			"id": i64, "hub_id": i64, "parent_id": i64, "name": str, "kind": str, "cidr": str, "description": str,
		}},
		{"sites", NewSitesDataSource, map[string]attr.Type{
			"id": i64, "name": str, "type": str, "description": str,
		}},
		{"subnets", NewSubnetsDataSource, map[string]attr.Type{
			"id": i64, "network_id": i64, "name": str, "cidr": str, "description": str, "zone_id": i64,
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var resp datasource.SchemaResponse
			tc.new().Schema(context.Background(), datasource.SchemaRequest{}, &resp)

			items, ok := resp.Schema.Attributes["items"].(schema.ListAttribute)
			if !ok {
				t.Fatalf("items is %T, want schema.ListAttribute", resp.Schema.Attributes["items"])
			}
			if !items.Computed {
				t.Error("items is not computed")
			}
			want := types.ObjectType{AttrTypes: tc.fields}
			if !items.ElementType.Equal(want) {
				t.Errorf("ElementType = %v, want %v", items.ElementType, want)
			}
			for name := range tc.fields {
				if !strings.Contains(items.Description, "`"+name+"` — ") {
					t.Errorf("description does not document %q: %q", name, items.Description)
				}
			}
		})
	}
}
