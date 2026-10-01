package resources

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// subnetZoneCase describes how each subnet resource creates and updates.
type subnetZoneCase struct {
	name       string
	newRes     func() resource.Resource
	createPath string
	createResp string // subnet inside zone 12
	cfg        map[string]any
	plan       map[string]any
	state      map[string]any
}

const (
	subnetInZone    = `{"id":40,"network_id":7,"name":"hp","cidr":"10.0.16.0/28","description":null,"zone_id":12}`
	subnetOutOfZone = `{"id":40,"network_id":7,"name":"hp","cidr":"10.0.16.0/28","description":null,"zone_id":null}`
)

func subnetZoneCases() []subnetZoneCase {
	alloc := func(name string, newRes func() resource.Resource, path string) subnetZoneCase {
		return subnetZoneCase{
			name: name, newRes: newRes, createPath: path, createResp: subnetInZone,
			cfg:   map[string]any{"network_id": int64(7), "prefix_length": int64(28), "name": "hp"},
			plan:  map[string]any{"id": unknown, "network_id": int64(7), "prefix_length": int64(28), "name": "hp", "description": unknown, "cidr": unknown, "zone_id": unknown},
			state: map[string]any{"id": int64(40), "network_id": int64(7), "prefix_length": int64(28), "name": "hp", "cidr": "10.0.16.0/28", "zone_id": int64(12)},
		}
	}
	return []subnetZoneCase{
		{
			name: "ipzilon_subnet", newRes: NewSubnetResource, createPath: "POST /subnets/", createResp: subnetInZone,
			cfg:   map[string]any{"network_id": int64(7), "name": "hp", "cidr": "10.0.16.0/28"},
			plan:  map[string]any{"id": unknown, "network_id": int64(7), "name": "hp", "cidr": "10.0.16.0/28", "description": unknown, "zone_id": unknown},
			state: map[string]any{"id": int64(40), "network_id": int64(7), "name": "hp", "cidr": "10.0.16.0/28", "zone_id": int64(12)},
		},
		alloc("ipzilon_next_subnet", NewNextSubnetResource, "POST /networks/7/next-available-subnet"),
		alloc("ipzilon_last_subnet", NewLastSubnetResource, "POST /networks/7/last-available-subnet"),
	}
}

func with(m map[string]any, k string, v any) map[string]any {
	out := map[string]any{}
	for key, val := range m {
		out[key] = val
	}
	out[k] = v
	return out
}

// TestSubnetZoneIDSentOnlyFromConfig: Create and Update send zone_id only when
// it is in the configuration, never a value that only comes from the state.
func TestSubnetZoneIDSentOnlyFromConfig(t *testing.T) {
	for _, tc := range subnetZoneCases() {
		t.Run(tc.name, func(t *testing.T) {
			c, api := newFakeAPI(t, "3.1.0", map[string]apiReply{
				tc.createPath:       {http.StatusCreated, tc.createResp},
				"PATCH /subnets/40": {http.StatusOK, subnetInZone},
			})
			h := newHarness(t, tc.newRes(), c)

			// Create with zone_id configured.
			resp := h.create(with(tc.cfg, "zone_id", int64(12)), with(tc.plan, "zone_id", int64(12)))
			if resp.Diagnostics.HasError() {
				t.Fatalf("Create: %v", resp.Diagnostics)
			}
			if got := api.Calls[0].Body["zone_id"]; got != float64(12) {
				t.Errorf("Create body zone_id = %v, want 12", got)
			}
			if got := attr(t, resp.State.Raw, "zone_id"); got != int64(12) {
				t.Errorf("state zone_id = %v, want 12", got)
			}

			// Create without zone_id: the key is not sent.
			api.Calls = nil
			if resp := h.create(tc.cfg, tc.plan); resp.Diagnostics.HasError() {
				t.Fatalf("Create: %v", resp.Diagnostics)
			}
			if _, ok := api.Calls[0].Body["zone_id"]; ok {
				t.Errorf("Create without zone_id sent %v", api.Calls[0].Body)
			}

			// Update with zone_id only in the state: not sent.
			api.Calls = nil
			cfg := with(tc.cfg, "description", "x")
			if resp := h.update(cfg, with(tc.state, "description", "x"), tc.state); resp.Diagnostics.HasError() {
				t.Fatalf("Update: %v", resp.Diagnostics)
			}
			if _, ok := api.Calls[0].Body["zone_id"]; ok {
				t.Errorf("Update sent a zone_id that is only in the state: %v", api.Calls[0].Body)
			}

			// Update with zone_id configured: sent.
			api.Calls = nil
			if resp := h.update(with(cfg, "zone_id", int64(12)), with(tc.state, "description", "x"), tc.state); resp.Diagnostics.HasError() {
				t.Fatalf("Update: %v", resp.Diagnostics)
			}
			if got := api.Calls[0].Body["zone_id"]; got != float64(12) {
				t.Errorf("Update body zone_id = %v, want 12", got)
			}
		})
	}
}

// TestSubnetZoneIgnoredByServer: a server that ignores zone_id (IPzilon 3.0.x
// reporting an unknown version) never leaves a subnet outside the requested
// zone: Create deletes it, Update reports the error without deleting.
func TestSubnetZoneIgnoredByServer(t *testing.T) {
	for _, tc := range subnetZoneCases() {
		t.Run(tc.name, func(t *testing.T) {
			c, api := newFakeAPI(t, "", map[string]apiReply{
				tc.createPath:        {http.StatusCreated, subnetOutOfZone},
				"DELETE /subnets/40": {http.StatusNoContent, ``},
				"PATCH /subnets/40":  {http.StatusOK, subnetOutOfZone},
			})
			h := newHarness(t, tc.newRes(), c)

			resp := h.create(with(tc.cfg, "zone_id", int64(12)), with(tc.plan, "zone_id", int64(12)))
			if !resp.Diagnostics.HasError() || !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "requires IPzilon >= 3.1.0") {
				t.Errorf("Create diagnostics = %v, want version error", resp.Diagnostics)
			}
			if last := api.Calls[len(api.Calls)-1]; last.Method != http.MethodDelete || last.Path != "/subnets/40" {
				t.Errorf("Create must roll back the subnet, last call = %v", last)
			}

			api.Calls = nil
			upd := h.update(with(tc.cfg, "zone_id", int64(12)), tc.state, tc.state)
			if !upd.Diagnostics.HasError() {
				t.Error("Update must report the ignored zone_id")
			}
			for _, call := range api.Calls {
				if call.Method == http.MethodDelete {
					t.Errorf("Update must not delete the subnet: %v", api.Calls)
				}
			}
		})
	}
}

// TestSubnetZoneOldIPzilon: with IPzilon 3.0.1 and zone_id configured, plan
// and apply fail without any request; without zone_id everything works.
func TestSubnetZoneOldIPzilon(t *testing.T) {
	for _, tc := range subnetZoneCases() {
		t.Run(tc.name, func(t *testing.T) {
			c, api := newFakeAPI(t, "3.0.1", map[string]apiReply{
				tc.createPath: {http.StatusCreated, subnetOutOfZone},
			})
			h := newHarness(t, tc.newRes(), c)
			cfg, plan := with(tc.cfg, "zone_id", int64(12)), with(tc.plan, "zone_id", int64(12))

			if !h.modifyPlan(cfg, plan, nil).Diagnostics.HasError() {
				t.Error("ModifyPlan must fail with zone_id on IPzilon 3.0.1")
			}
			if !h.create(cfg, plan).Diagnostics.HasError() {
				t.Error("Create must fail with zone_id on IPzilon 3.0.1")
			}
			if len(api.Calls) != 0 {
				t.Errorf("no request expected, got %v", api.Calls)
			}
			if h.modifyPlan(tc.cfg, tc.plan, nil).Diagnostics.HasError() {
				t.Error("ModifyPlan without zone_id must work on IPzilon 3.0.1")
			}
			if resp := h.create(tc.cfg, tc.plan); resp.Diagnostics.HasError() {
				t.Errorf("Create without zone_id must work on IPzilon 3.0.1: %v", resp.Diagnostics)
			}
		})
	}
}

// TestZoneIDFollowsCIDR: in ipzilon_subnet an unconfigured zone_id keeps the
// state value while the CIDR does not change and becomes unknown otherwise.
func TestZoneIDFollowsCIDR(t *testing.T) {
	c, _ := newFakeAPI(t, "3.1.0", nil)
	h := newHarness(t, NewSubnetResource(), c)
	state := map[string]any{"id": int64(40), "network_id": int64(7), "name": "hp", "cidr": "10.0.16.0/28", "zone_id": int64(12)}
	cases := map[string]struct {
		state map[string]any
		cidr  string
		cfg   any
		want  any
	}{
		"same cidr keeps state":      {state, "10.0.16.0/28", nil, int64(12)},
		"same cidr keeps null state": {with(state, "zone_id", nil), "10.0.16.0/28", nil, nil},
		"new cidr is unknown":        {state, "10.0.20.0/28", nil, unknown},
		"create stays unknown":       {nil, "10.0.16.0/28", nil, unknown},
		"configured value untouched": {state, "10.0.20.0/28", int64(12), int64(12)},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			planVals := with(with(state, "cidr", tc.cidr), "zone_id", unknown)
			if tc.cfg != nil {
				planVals["zone_id"] = tc.cfg
			}
			req := planmodifier.Int64Request{
				Path:        path.Root("zone_id"),
				Config:      h.config(with(with(state, "cidr", tc.cidr), "zone_id", tc.cfg)),
				Plan:        h.plan(planVals),
				State:       h.emptyState(),
				ConfigValue: types.Int64Null(),
				PlanValue:   types.Int64Unknown(),
				StateValue:  types.Int64Null(),
			}
			if tc.state != nil {
				req.State = h.state(tc.state)
				if z, ok := tc.state["zone_id"].(int64); ok {
					req.StateValue = types.Int64Value(z)
				}
			}
			if n, ok := tc.cfg.(int64); ok {
				req.ConfigValue = types.Int64Value(n)
				req.PlanValue = types.Int64Value(n)
			}
			resp := &planmodifier.Int64Response{PlanValue: req.PlanValue}
			zoneIDFollowsCIDR().PlanModifyInt64(context.Background(), req, resp)
			if resp.Diagnostics.HasError() {
				t.Fatalf("diagnostics: %v", resp.Diagnostics)
			}
			var got any
			switch {
			case resp.PlanValue.IsUnknown():
				got = unknown
			case resp.PlanValue.IsNull():
				got = nil
			default:
				got = resp.PlanValue.ValueInt64()
			}
			if got != tc.want {
				t.Errorf("planned zone_id = %v, want %v", got, tc.want)
			}
		})
	}
}
