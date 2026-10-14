package datasources

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// itemField describes one attribute of the objects returned in items.
type itemField struct {
	name        string
	typ         attr.Type
	description string
}

// itemsAttribute declares items as a plain list of objects instead of a
// ListNestedAttribute: terraform test cannot mock computed nested lists
// (hashicorp/terraform internal/moduletest/mocking/fill.go), so the field
// descriptions are folded into the items description.
func itemsAttribute(summary string, fields []itemField) schema.ListAttribute {
	attrTypes := make(map[string]attr.Type, len(fields))
	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		attrTypes[f.name] = f.typ
		parts = append(parts, "`"+f.name+"` — "+f.description)
	}
	return schema.ListAttribute{
		Computed:    true,
		ElementType: types.ObjectType{AttrTypes: attrTypes},
		Description: summary + " Each item has: " + strings.Join(parts, " "),
	}
}
