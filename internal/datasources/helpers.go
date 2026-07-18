package datasources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/mdepedrof/terraform-provider-ipzilon/internal/client"
)

func configureClient(req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) *client.Client {
	if req.ProviderData == nil {
		return nil
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("got %T", req.ProviderData))
		return nil
	}
	return c
}

func validateFilters(ctx context.Context, hasID, hasParent bool, resp *datasource.ReadResponse) bool {
	if hasID && hasParent {
		resp.Diagnostics.AddError("Conflicting filters", "Provide either id or a parent filter, not both.")
		return false
	}
	if !hasID && !hasParent {
		resp.Diagnostics.AddError("Missing filter", "Provide either id or a parent filter.")
		return false
	}
	return true
}
