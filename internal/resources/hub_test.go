package resources

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func hubVals(site int64, name string) map[string]any {
	return map[string]any{"id": int64(5), "site_id": site, "name": name, "address_space": "10.0.0.0/16", "location": nil, "description": nil}
}

func TestHubSiteIDDoesNotRequireReplace(t *testing.T) {
	var resp resource.SchemaResponse
	NewHubResource().Schema(context.Background(), resource.SchemaRequest{}, &resp)
	siteID := resp.Schema.Attributes["site_id"].(schema.Int64Attribute)
	if len(siteID.PlanModifiers) != 0 {
		t.Errorf("site_id plan modifiers = %v, want none (moving a hub is an in-place update)", siteID.PlanModifiers)
	}
}

func TestHubModifyPlanMoveRequiresIPzilon32(t *testing.T) {
	cases := map[string]struct {
		version     string
		state, plan map[string]any
		wantErr     bool
	}{
		"move on 3.1.0":         {"3.1.0", hubVals(1, "hub"), hubVals(2, "hub"), true},
		"move on 3.2.0":         {"3.2.0", hubVals(1, "hub"), hubVals(2, "hub"), false},
		"move, unknown version": {"", hubVals(1, "hub"), hubVals(2, "hub"), false},
		"rename on 3.1.0":       {"3.1.0", hubVals(1, "hub"), hubVals(1, "renamed"), false},
		"create on 3.1.0":       {"3.1.0", nil, hubVals(1, "hub"), false},
		// A hub moved outside Terraform is moved back in place, never replaced.
		"drift back on 3.2.0": {"3.2.0", hubVals(2, "hub"), hubVals(1, "hub"), false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c, api := newFakeAPI(t, tc.version, nil)
			h := newHarness(t, NewHubResource(), c)
			resp := h.modifyPlan(tc.plan, tc.plan, tc.state)
			if resp.Diagnostics.HasError() != tc.wantErr {
				t.Fatalf("diagnostics = %v, wantErr %v", resp.Diagnostics, tc.wantErr)
			}
			if tc.wantErr && !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "moving ipzilon_hub to another site requires IPzilon >= 3.2.0") {
				t.Errorf("detail = %q", resp.Diagnostics.Errors()[0].Detail())
			}
			if len(resp.RequiresReplace) != 0 {
				t.Errorf("RequiresReplace = %v, want none", resp.RequiresReplace)
			}
			if len(api.Calls) != 0 {
				t.Errorf("no request expected, got %v", api.Calls)
			}
		})
	}
}

func TestHubUpdateMovesSite(t *testing.T) {
	c, api := newFakeAPI(t, "3.2.0", map[string]apiReply{
		"PATCH /hubs/5": {http.StatusOK, `{"id":5,"site_id":2,"name":"hub","address_space":"10.0.0.0/16","location":null,"description":null}`},
	})
	h := newHarness(t, NewHubResource(), c)
	resp := h.update(hubVals(2, "hub"), hubVals(2, "hub"), hubVals(1, "hub"))
	if resp.Diagnostics.HasError() {
		t.Fatalf("Update: %v", resp.Diagnostics)
	}
	if got := api.Calls[0].Body["site_id"]; got != float64(2) {
		t.Errorf("site_id sent = %v, want 2 (body %v)", got, api.Calls[0].Body)
	}
	if attr(t, resp.State.Raw, "site_id") != int64(2) || attr(t, resp.State.Raw, "id") != int64(5) {
		t.Errorf("state = %v", resp.State.Raw)
	}
}

func TestHubUpdateWithoutMoveDoesNotSendSiteID(t *testing.T) {
	c, api := newFakeAPI(t, "3.1.0", map[string]apiReply{
		"PATCH /hubs/5": {http.StatusOK, `{"id":5,"site_id":1,"name":"renamed","address_space":"10.0.0.0/16","location":null,"description":null}`},
	})
	h := newHarness(t, NewHubResource(), c)
	resp := h.update(hubVals(1, "renamed"), hubVals(1, "renamed"), hubVals(1, "hub"))
	if resp.Diagnostics.HasError() {
		t.Fatalf("Update: %v", resp.Diagnostics)
	}
	if _, ok := api.Calls[0].Body["site_id"]; ok {
		t.Errorf("site_id must not be sent when the hub does not move, body = %v", api.Calls[0].Body)
	}
}

func TestHubUpdateServerIgnoresSiteID(t *testing.T) {
	c, _ := newFakeAPI(t, "", map[string]apiReply{
		"PATCH /hubs/5": {http.StatusOK, `{"id":5,"site_id":1,"name":"hub","address_space":"10.0.0.0/16","location":null,"description":null}`},
	})
	h := newHarness(t, NewHubResource(), c)
	resp := h.update(hubVals(2, "hub"), hubVals(2, "hub"), hubVals(1, "hub"))
	if !resp.Diagnostics.HasError() || !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "the server ignored site_id") {
		t.Fatalf("diagnostics = %v", resp.Diagnostics)
	}
	if got := attr(t, resp.State.Raw, "site_id"); got != int64(1) {
		t.Errorf("state site_id = %v, want the real site 1", got)
	}
}

func TestHubUpdateMoveRejectedShowsAPIError(t *testing.T) {
	c, _ := newFakeAPI(t, "3.2.0", map[string]apiReply{
		"PATCH /hubs/5": {http.StatusConflict, `{"detail":"Hub name 'hub' already exists in site 2"}`},
	})
	h := newHarness(t, NewHubResource(), c)
	resp := h.update(hubVals(2, "hub"), hubVals(2, "hub"), hubVals(1, "hub"))
	if !resp.Diagnostics.HasError() {
		t.Fatal("want an error")
	}
	d := resp.Diagnostics.Errors()[0]
	if d.Summary() != "Update hub failed" || !strings.Contains(d.Detail(), "Hub name 'hub' already exists in site 2") {
		t.Errorf("diagnostic = %s: %s", d.Summary(), d.Detail())
	}
}
