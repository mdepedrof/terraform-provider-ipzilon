package resources

import (
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

const allocZoneJSON = `{"id":13,"network_id":7,"name":"personal_zone","cidr":"10.0.19.128/25","description":null}`

func allocZoneCfg() map[string]any {
	return map[string]any{"network_id": int64(7), "prefix_length": int64(25), "name": "personal_zone"}
}

func allocZonePlan() map[string]any {
	return map[string]any{"id": unknown, "network_id": int64(7), "prefix_length": int64(25), "name": "personal_zone", "description": unknown, "cidr": unknown}
}

func allocZoneState() map[string]any {
	return map[string]any{"id": int64(13), "network_id": int64(7), "prefix_length": int64(25), "name": "personal_zone", "cidr": "10.0.19.128/25"}
}

func TestAllocNetworkZoneCreate(t *testing.T) {
	for _, tc := range []struct {
		newRes func() resource.Resource
		path   string
	}{
		{NewNextNetworkZoneResource, "/networks/7/next-available-zone"},
		{NewLastNetworkZoneResource, "/networks/7/last-available-zone"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			c, api := newFakeAPI(t, "3.1.0", map[string]apiReply{"POST " + tc.path: {http.StatusCreated, allocZoneJSON}})
			h := newHarness(t, tc.newRes(), c)
			resp := h.create(allocZoneCfg(), allocZonePlan())
			if resp.Diagnostics.HasError() {
				t.Fatalf("Create: %v", resp.Diagnostics)
			}
			body := api.Calls[0].Body
			if body["prefix_length"] != float64(25) || body["name"] != "personal_zone" {
				t.Errorf("body = %v", body)
			}
			if got := attr(t, resp.State.Raw, "cidr"); got != "10.0.19.128/25" {
				t.Errorf("cidr = %v", got)
			}
			if got := attr(t, resp.State.Raw, "prefix_length"); got != int64(25) {
				t.Errorf("prefix_length = %v", got)
			}
		})
	}
}

func TestAllocNetworkZoneUpdateSendsNoCIDR(t *testing.T) {
	c, api := newFakeAPI(t, "3.1.0", map[string]apiReply{
		"PATCH /zones/13": {http.StatusOK, `{"id":13,"network_id":7,"name":"renamed","cidr":"10.0.19.128/25","description":"d"}`},
	})
	h := newHarness(t, NewNextNetworkZoneResource(), c)
	plan := with(with(allocZoneState(), "name", "renamed"), "description", "d")
	resp := h.update(plan, plan, allocZoneState())
	if resp.Diagnostics.HasError() {
		t.Fatalf("Update: %v", resp.Diagnostics)
	}
	body := api.Calls[0].Body
	if _, ok := body["cidr"]; ok || body["name"] != "renamed" || body["description"] != "d" {
		t.Errorf("body = %v, want name and description only", body)
	}
}

func TestAllocNetworkZoneErrors(t *testing.T) {
	cases := map[string]struct {
		reply apiReply
		want  string
	}{
		"truncated": {apiReply{http.StatusConflict, `{"detail":"Search truncated after 4096 steps without finding a free /25 block for a zone in 10.0.16.0/22"}`}, "ipzilon_network_zone"},
		"no block":  {apiReply{http.StatusConflict, `{"detail":"No free /25 block available for a zone in 10.0.16.0/22"}`}, "No free /25 block available for a zone in 10.0.16.0/22"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c, _ := newFakeAPI(t, "3.1.0", map[string]apiReply{"POST /networks/7/next-available-zone": tc.reply})
			h := newHarness(t, NewNextNetworkZoneResource(), c)
			resp := h.create(allocZoneCfg(), allocZonePlan())
			if !resp.Diagnostics.HasError() || !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), tc.want) {
				t.Errorf("diagnostics = %v, want %q", resp.Diagnostics, tc.want)
			}
		})
	}
}

func TestAllocNetworkZoneOldIPzilon(t *testing.T) {
	c, api := newFakeAPI(t, "3.0.1", nil)
	h := newHarness(t, NewLastNetworkZoneResource(), c)
	if !h.modifyPlan(allocZoneCfg(), allocZonePlan(), nil).Diagnostics.HasError() {
		t.Error("ModifyPlan must fail on IPzilon 3.0.1")
	}
	if !h.create(allocZoneCfg(), allocZonePlan()).Diagnostics.HasError() {
		t.Error("Create must fail on IPzilon 3.0.1")
	}
	if len(api.Calls) != 0 {
		t.Errorf("no request expected, got %v", api.Calls)
	}
}
