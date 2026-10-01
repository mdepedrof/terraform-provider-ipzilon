package resources

import (
	"net/http"
	"strings"
	"testing"
)

const zoneJSON = `{"id":12,"network_id":7,"name":"pooled_zone_1","cidr":"10.0.16.0/23","description":"pooled","total_ips":512,"used_ips":32,"available_ips":480,"alert_percent":6,"alert_metric":"block_alloc","subnet_count":2}`

func zoneVals() map[string]any {
	return map[string]any{"id": int64(12), "network_id": int64(7), "name": "pooled_zone_1", "cidr": "10.0.16.0/23", "description": "pooled"}
}

func TestNetworkZoneCreate(t *testing.T) {
	c, api := newFakeAPI(t, "3.1.0", map[string]apiReply{
		"POST /networks/7/zones": {http.StatusCreated, zoneJSON},
	})
	h := newHarness(t, NewNetworkZoneResource(), c)
	cfg := map[string]any{"network_id": int64(7), "name": "pooled_zone_1", "cidr": "10.0.16.0/23", "description": "pooled"}
	plan := map[string]any{"id": unknown, "network_id": int64(7), "name": "pooled_zone_1", "cidr": "10.0.16.0/23", "description": "pooled"}
	resp := h.create(cfg, plan)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Create: %v", resp.Diagnostics)
	}
	body := api.Calls[0].Body
	if body["name"] != "pooled_zone_1" || body["cidr"] != "10.0.16.0/23" || body["description"] != "pooled" {
		t.Errorf("body = %v", body)
	}
	if _, ok := body["network_id"]; ok {
		t.Errorf("network_id must go in the path, body = %v", body)
	}
	for name, want := range zoneVals() {
		if got := attr(t, resp.State.Raw, name); got != want {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
}

func TestNetworkZoneUpdate(t *testing.T) {
	c, api := newFakeAPI(t, "3.1.0", map[string]apiReply{
		"PATCH /zones/12": {http.StatusOK, `{"id":12,"network_id":7,"name":"renamed","cidr":"10.0.16.0/22","description":null}`},
	})
	h := newHarness(t, NewNetworkZoneResource(), c)
	plan := map[string]any{"id": int64(12), "network_id": int64(7), "name": "renamed", "cidr": "10.0.16.0/22", "description": nil}
	resp := h.update(plan, plan, zoneVals())
	if resp.Diagnostics.HasError() {
		t.Fatalf("Update: %v", resp.Diagnostics)
	}
	body := api.Calls[0].Body
	if body["name"] != "renamed" || body["cidr"] != "10.0.16.0/22" {
		t.Errorf("body = %v", body)
	}
	if d, ok := body["description"]; !ok || d != nil {
		t.Errorf("description must be sent as null to clear it, body = %v", body)
	}
	if got := attr(t, resp.State.Raw, "description"); got != nil {
		t.Errorf("description = %v, want null", got)
	}
}

func TestNetworkZoneReadNotFoundRemovesFromState(t *testing.T) {
	c, _ := newFakeAPI(t, "3.1.0", map[string]apiReply{
		"GET /zones/12": {http.StatusNotFound, `{"detail":"Zone not found"}`},
	})
	h := newHarness(t, NewNetworkZoneResource(), c)
	resp := h.read(zoneVals())
	if resp.Diagnostics.HasError() {
		t.Fatalf("Read: %v", resp.Diagnostics)
	}
	if !resp.State.Raw.IsNull() {
		t.Error("Read on 404 must remove the zone from state")
	}
}

func TestNetworkZoneDeleteNotFoundIsOK(t *testing.T) {
	c, _ := newFakeAPI(t, "3.1.0", map[string]apiReply{
		"DELETE /zones/12": {http.StatusNotFound, `{"detail":"Zone not found"}`},
	})
	h := newHarness(t, NewNetworkZoneResource(), c)
	if resp := h.delete(zoneVals()); resp.Diagnostics.HasError() {
		t.Fatalf("Delete: %v", resp.Diagnostics)
	}
}

func TestNetworkZoneAPIErrorIsShown(t *testing.T) {
	c, _ := newFakeAPI(t, "3.1.0", map[string]apiReply{
		"POST /networks/7/zones": {http.StatusBadRequest, `{"detail":"Zone 10.0.20.0/24 is not within network 10.0.16.0/22"}`},
	})
	h := newHarness(t, NewNetworkZoneResource(), c)
	cfg := map[string]any{"network_id": int64(7), "name": "z", "cidr": "10.0.20.0/24"}
	resp := h.create(cfg, map[string]any{"id": unknown, "network_id": int64(7), "name": "z", "cidr": "10.0.20.0/24", "description": unknown})
	if !resp.Diagnostics.HasError() || !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "Zone 10.0.20.0/24 is not within network 10.0.16.0/22") {
		t.Errorf("diagnostics = %v, want the API detail", resp.Diagnostics)
	}
}

func TestNetworkZoneOldIPzilonFailsWithoutRequests(t *testing.T) {
	c, api := newFakeAPI(t, "3.0.1", nil)
	h := newHarness(t, NewNetworkZoneResource(), c)
	cfg := map[string]any{"network_id": int64(7), "name": "z", "cidr": "10.0.16.0/23"}
	plan := map[string]any{"id": unknown, "network_id": int64(7), "name": "z", "cidr": "10.0.16.0/23", "description": unknown}

	checks := map[string]bool{
		"ModifyPlan": h.modifyPlan(cfg, plan, nil).Diagnostics.HasError(),
		"Create":     h.create(cfg, plan).Diagnostics.HasError(),
		"Read":       h.read(zoneVals()).Diagnostics.HasError(),
	}
	for op, failed := range checks {
		if !failed {
			t.Errorf("%s against IPzilon 3.0.1 must fail", op)
		}
	}
	if len(api.Calls) != 0 {
		t.Errorf("no request expected, got %v", api.Calls)
	}
}

// TestNetworkZoneUnknownVersionWithoutZoneRoutes: an IPzilon 3.0.x reporting
// an unknown version answers the zone routes with the generic 404. Create
// reports the version error and Read keeps the zone in the state instead of
// silently removing it.
func TestNetworkZoneUnknownVersionWithoutZoneRoutes(t *testing.T) {
	c, _ := newFakeAPI(t, "0.0.0-dev", map[string]apiReply{
		"POST /networks/7/zones": {http.StatusNotFound, `{"detail":"Not Found"}`},
		"GET /zones/12":          {http.StatusNotFound, `{"detail":"Not Found"}`},
	})
	h := newHarness(t, NewNetworkZoneResource(), c)
	cfg := map[string]any{"network_id": int64(7), "name": "z", "cidr": "10.0.16.0/23"}
	create := h.create(cfg, map[string]any{"id": unknown, "network_id": int64(7), "name": "z", "cidr": "10.0.16.0/23", "description": unknown})
	if !create.Diagnostics.HasError() || !strings.Contains(create.Diagnostics.Errors()[0].Detail(), "requires IPzilon >= 3.1.0") {
		t.Errorf("Create diagnostics = %v, want version error", create.Diagnostics)
	}
	read := h.read(zoneVals())
	if !read.Diagnostics.HasError() || read.State.Raw.IsNull() {
		t.Errorf("Read must fail with the version error and keep the state: %v", read.Diagnostics)
	}
}
