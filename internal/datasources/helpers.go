package datasources

import (
	"context"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/mdepedrof/terraform-provider-ipzilon/internal/client"
)

// stringFilter converts a possibly null/unknown types.String config value
// into a *string suitable for building an optional query filter.
func stringFilter(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	s := v.ValueString()
	return &s
}

// hubScopesURL builds the request URL for GET /hubs/{hub_id}/scopes with the
// optional server-side filters parent_id/root_only (mutually exclusive,
// validated by the caller) and cidr/name.
func hubScopesURL(hubID int64, hasParent bool, parentID int64, rootOnly bool, cidr, name *string) string {
	q := url.Values{}
	if hasParent {
		q.Set("parent_id", fmt.Sprintf("%d", parentID))
	} else if rootOnly {
		q.Set("root_only", "true")
	}
	if cidr != nil {
		q.Set("cidr", *cidr)
	}
	if name != nil {
		q.Set("name", *name)
	}

	reqURL := fmt.Sprintf("/hubs/%d/scopes", hubID)
	if encoded := q.Encode(); encoded != "" {
		reqURL += "?" + encoded
	}
	return reqURL
}

// hubNetworksURL builds the request URL for GET /hubs/{hub_id}/networks with
// the optional server-side filters cidr/name.
func hubNetworksURL(hubID int64, cidr, name *string) string {
	q := url.Values{}
	if cidr != nil {
		q.Set("cidr", *cidr)
	}
	if name != nil {
		q.Set("name", *name)
	}

	reqURL := fmt.Sprintf("/hubs/%d/networks", hubID)
	if encoded := q.Encode(); encoded != "" {
		reqURL += "?" + encoded
	}
	return reqURL
}

// scopeNetworksURL builds the request URL for GET /scopes/{scope_id}/networks
// with the optional server-side filters cidr/name.
func scopeNetworksURL(scopeID int64, cidr, name *string) string {
	q := url.Values{}
	if cidr != nil {
		q.Set("cidr", *cidr)
	}
	if name != nil {
		q.Set("name", *name)
	}

	reqURL := fmt.Sprintf("/scopes/%d/networks", scopeID)
	if encoded := q.Encode(); encoded != "" {
		reqURL += "?" + encoded
	}
	return reqURL
}

// siteHubsURL builds the request URL for GET /sites/{site_id}/hubs with the
// optional server-side filters address_space/name.
func siteHubsURL(siteID int64, addressSpace, name *string) string {
	q := url.Values{}
	if addressSpace != nil {
		q.Set("address_space", *addressSpace)
	}
	if name != nil {
		q.Set("name", *name)
	}

	reqURL := fmt.Sprintf("/sites/%d/hubs", siteID)
	if encoded := q.Encode(); encoded != "" {
		reqURL += "?" + encoded
	}
	return reqURL
}

// sitesURL builds the request URL for GET /sites/ with the optional
// server-side filter name.
func sitesURL(name *string) string {
	q := url.Values{}
	if name != nil {
		q.Set("name", *name)
	}

	reqURL := "/sites/"
	if encoded := q.Encode(); encoded != "" {
		reqURL += "?" + encoded
	}
	return reqURL
}

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
