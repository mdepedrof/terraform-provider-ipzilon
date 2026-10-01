package resources_test

import (
	"context"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/mdepedrof/terraform-provider-ipzilon/internal/client"
	"github.com/mdepedrof/terraform-provider-ipzilon/internal/provider"
	"github.com/mdepedrof/terraform-provider-ipzilon/internal/resources"
)

// importCase describes one resource of the read-after-import table: how to
// build it and what the IPzilon API answers for its GET endpoint.
type importCase struct {
	name        string // resource type name, e.g. ipzilon_next_ip_address
	newResource func() resource.Resource
	id          int64
	path        string // GET path the resource reads
	body        string // JSON the API answers with
	// want holds attribute values that must appear in the state after Read.
	want map[string]any
}

// importCases is the table of every resource registered in the provider.
// TestReadAfterImportCoversAllResources fails when a resource is missing here.
var importCases = []importCase{
	{
		name:        "ipzilon_next_ip_address",
		newResource: resources.NewNextIPAddressResource,
		id:          2204,
		path:        "/ips/2204",
		body:        `{"id":2204,"subnet_id":65,"address":"10.1.4.10","status":"reserved","is_azure_reserved":false,"hostname":"ilb-events","description":"ILB events"}`,
		want:        map[string]any{"subnet_id": int64(65), "address": "10.1.4.10"},
	},
	{
		name:        "ipzilon_ip_address",
		newResource: resources.NewIPAddressResource,
		id:          77,
		path:        "/ips/77",
		body:        `{"id":77,"subnet_id":5,"address":"10.0.1.17","status":"used","is_azure_reserved":false,"hostname":"web","description":null}`,
		want:        map[string]any{"subnet_id": int64(5), "address": "10.0.1.17", "status": "used"},
	},
	{
		name:        "ipzilon_next_subnet",
		newResource: resources.NewNextSubnetResource,
		id:          11,
		path:        "/subnets/11",
		body:        `{"id":11,"network_id":3,"name":"app","cidr":"10.1.4.0/24","description":"app subnet","zone_id":30}`,
		want:        map[string]any{"network_id": int64(3), "prefix_length": int64(24), "cidr": "10.1.4.0/24", "zone_id": int64(30)},
	},
	{
		name:        "ipzilon_last_subnet",
		newResource: resources.NewLastSubnetResource,
		id:          12,
		path:        "/subnets/12",
		body:        `{"id":12,"network_id":3,"name":"tail","cidr":"10.1.255.192/26","description":null,"zone_id":null}`,
		want:        map[string]any{"network_id": int64(3), "prefix_length": int64(26), "zone_id": nil},
	},
	{
		name:        "ipzilon_next_network",
		newResource: resources.NewNextNetworkResource,
		id:          21,
		path:        "/networks/21",
		body:        `{"id":21,"scope_id":8,"name":"net","cidr":"10.2.0.0/22","description":null}`,
		want:        map[string]any{"scope_id": int64(8), "prefix_length": int64(22), "cidr": "10.2.0.0/22"},
	},
	{
		name:        "ipzilon_hub",
		newResource: resources.NewHubResource,
		id:          1,
		path:        "/hubs/1",
		body:        `{"id":1,"site_id":2,"name":"hub-a","address_space":"10.0.0.0/8","location":"westeurope","description":null}`,
		want:        map[string]any{"site_id": int64(2), "name": "hub-a", "description": nil},
	},
	{
		name:        "ipzilon_scope",
		newResource: resources.NewScopeResource,
		id:          4,
		path:        "/scopes/4",
		body:        `{"id":4,"hub_id":1,"parent_id":null,"name":"lz","kind":"landing_zone","cidr":"10.1.0.0/16","description":null}`,
		want:        map[string]any{"hub_id": int64(1), "kind": "landing_zone"},
	},
	{
		name:        "ipzilon_network",
		newResource: resources.NewNetworkResource,
		id:          6,
		path:        "/networks/6",
		body:        `{"id":6,"scope_id":4,"name":"vnet","cidr":"10.1.0.0/20","description":null}`,
		want:        map[string]any{"scope_id": int64(4), "cidr": "10.1.0.0/20"},
	},
	{
		name:        "ipzilon_subnet",
		newResource: resources.NewSubnetResource,
		id:          9,
		path:        "/subnets/9",
		body:        `{"id":9,"network_id":6,"name":"sn","cidr":"10.1.0.0/24","description":null,"zone_id":30}`,
		want:        map[string]any{"network_id": int64(6), "cidr": "10.1.0.0/24", "description": nil, "zone_id": int64(30)},
	},
	{
		// The usage metrics of NetworkZoneResponse are in the body to check
		// that they are ignored (the provider does not expose them).
		name:        "ipzilon_network_zone",
		newResource: resources.NewNetworkZoneResource,
		id:          12,
		path:        "/zones/12",
		body:        `{"id":12,"network_id":7,"name":"pooled_zone_1","cidr":"10.0.16.0/23","description":null,"total_ips":512,"used_ips":32,"available_ips":480,"alert_percent":6,"alert_metric":"block_alloc","subnet_count":2}`,
		want:        map[string]any{"network_id": int64(7), "name": "pooled_zone_1", "cidr": "10.0.16.0/23", "description": nil},
	},
	{
		name:        "ipzilon_next_network_zone",
		newResource: resources.NewNextNetworkZoneResource,
		id:          13,
		path:        "/zones/13",
		body:        `{"id":13,"network_id":7,"name":"personal_zone","cidr":"10.0.19.128/25","description":"personal","total_ips":128,"used_ips":0,"available_ips":128,"alert_percent":0,"alert_metric":"block_alloc","subnet_count":0}`,
		want:        map[string]any{"network_id": int64(7), "prefix_length": int64(25), "cidr": "10.0.19.128/25"},
	},
	{
		name:        "ipzilon_last_network_zone",
		newResource: resources.NewLastNetworkZoneResource,
		id:          14,
		path:        "/zones/14",
		body:        `{"id":14,"network_id":7,"name":"tail_zone","cidr":"10.0.19.0/26","description":null}`,
		want:        map[string]any{"network_id": int64(7), "prefix_length": int64(26), "name": "tail_zone"},
	},
}

// newAPIServer answers body to GET path and 404 to anything else.
func newAPIServer(t *testing.T, path, body string) *client.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"detail":"not found"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return &client.Client{BaseURL: srv.URL, Token: "test", HTTPClient: http.DefaultClient}
}

func configured(t *testing.T, tc importCase, c *client.Client) (resource.Resource, resource.SchemaResponse) {
	t.Helper()
	r := tc.newResource()
	var schemaResp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &schemaResp)
	var cfgResp resource.ConfigureResponse
	r.(resource.ResourceWithConfigure).Configure(context.Background(), resource.ConfigureRequest{ProviderData: c}, &cfgResp)
	if cfgResp.Diagnostics.HasError() {
		t.Fatalf("Configure: %v", cfgResp.Diagnostics)
	}
	return r, schemaResp
}

// stateWithOnlyID builds the state Terraform hands to Read right after an
// import: every attribute null except id.
func stateWithOnlyID(t *testing.T, schemaResp resource.SchemaResponse, id int64) tfsdk.State {
	t.Helper()
	objType := schemaResp.Schema.Type().TerraformType(context.Background()).(tftypes.Object)
	vals := make(map[string]tftypes.Value, len(objType.AttributeTypes))
	for name, typ := range objType.AttributeTypes {
		vals[name] = tftypes.NewValue(typ, nil)
	}
	vals["id"] = tftypes.NewValue(tftypes.Number, big.NewFloat(float64(id)))
	return tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(objType, vals)}
}

func attrValues(t *testing.T, state tfsdk.State) map[string]tftypes.Value {
	t.Helper()
	vals := map[string]tftypes.Value{}
	if err := state.Raw.As(&vals); err != nil {
		t.Fatalf("state is not an object: %v", err)
	}
	return vals
}

// assertComplete fails when a Required attribute is null or any attribute is
// unknown after Read (contract C2).
func assertComplete(t *testing.T, name string, schemaResp resource.SchemaResponse, state tfsdk.State) {
	t.Helper()
	vals := attrValues(t, state)
	for attr, def := range schemaResp.Schema.Attributes {
		v, ok := vals[attr]
		if !ok {
			t.Errorf("%s.%s: missing from state", name, attr)
			continue
		}
		if !v.IsFullyKnown() {
			t.Errorf("%s.%s: unknown after Read", name, attr)
		}
		if def.IsRequired() && v.IsNull() {
			t.Errorf("%s.%s: Required attribute is null after Read", name, attr)
		}
	}
}

func assertWant(t *testing.T, name string, vals map[string]tftypes.Value, want map[string]any) {
	t.Helper()
	for attr, w := range want {
		v, ok := vals[attr]
		if !ok {
			t.Errorf("%s.%s: missing from state", name, attr)
			continue
		}
		switch exp := w.(type) {
		case nil:
			if !v.IsNull() {
				t.Errorf("%s.%s = %s, want null", name, attr, v)
			}
		case int64:
			var f big.Float
			if err := v.As(&f); err != nil {
				t.Errorf("%s.%s: %v", name, attr, err)
				continue
			}
			if got, _ := f.Int64(); got != exp {
				t.Errorf("%s.%s = %d, want %d", name, attr, got, exp)
			}
		case string:
			var got string
			if err := v.As(&got); err != nil {
				t.Errorf("%s.%s: %v", name, attr, err)
				continue
			}
			if got != exp {
				t.Errorf("%s.%s = %q, want %q", name, attr, got, exp)
			}
		}
	}
}

// TestReadAfterImport starts from a state that only has the id (what
// `terraform import` leaves) and checks that Read completes every attribute.
func TestReadAfterImport(t *testing.T) {
	for _, tc := range importCases {
		t.Run(tc.name, func(t *testing.T) {
			r, schemaResp := configured(t, tc, newAPIServer(t, tc.path, tc.body))
			resp := resource.ReadResponse{State: stateWithOnlyID(t, schemaResp, tc.id)}
			r.Read(context.Background(), resource.ReadRequest{State: stateWithOnlyID(t, schemaResp, tc.id)}, &resp)
			if resp.Diagnostics.HasError() {
				t.Fatalf("Read: %v", resp.Diagnostics)
			}
			if resp.State.Raw.IsNull() {
				t.Fatal("Read removed the resource from state")
			}
			assertComplete(t, tc.name, schemaResp, resp.State)
			assertWant(t, tc.name, attrValues(t, resp.State), tc.want)
		})
	}
}

// TestReadAfterImportCoversAllResources fails when a resource registered in
// the provider has no row in importCases (or the table has a stale row).
func TestReadAfterImportCoversAllResources(t *testing.T) {
	registered := map[string]bool{}
	p := provider.New("test")()
	for _, newRes := range p.Resources(context.Background()) {
		var md resource.MetadataResponse
		newRes().Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "ipzilon"}, &md)
		registered[md.TypeName] = true
	}
	inTable := map[string]bool{}
	for _, tc := range importCases {
		inTable[tc.name] = true
	}
	var missing, stale []string
	for name := range registered {
		if !inTable[name] {
			missing = append(missing, name)
		}
	}
	for name := range inTable {
		if !registered[name] {
			stale = append(stale, name)
		}
	}
	sort.Strings(missing)
	sort.Strings(stale)
	if len(missing) > 0 {
		t.Errorf("resources without a row in importCases (add them): %s", strings.Join(missing, ", "))
	}
	if len(stale) > 0 {
		t.Errorf("importCases rows for resources no longer registered: %s", strings.Join(stale, ", "))
	}
}

// anyPathServer answers every request with body, whatever the method or path.
func anyPathServer(t *testing.T, body string) *client.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return &client.Client{BaseURL: srv.URL, Token: "test", HTTPClient: http.DefaultClient}
}

// readState runs Read from an id-only state against an API that answers body
// and returns the resulting state.
func readState(t *testing.T, tc importCase, body string) tfsdk.State {
	t.Helper()
	r, schemaResp := configured(t, tc, anyPathServer(t, body))
	resp := resource.ReadResponse{State: stateWithOnlyID(t, schemaResp, tc.id)}
	r.Read(context.Background(), resource.ReadRequest{State: stateWithOnlyID(t, schemaResp, tc.id)}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Read: %v", resp.Diagnostics)
	}
	return resp.State
}

// TestStateMatchesAcrossOperations checks, for all nine resources, that Create,
// Read and Update leave exactly the same state for the same API object
// (FR-004): the state comes only from the API response, never from a mix with
// the plan or the previous state.
func TestStateMatchesAcrossOperations(t *testing.T) {
	ctx := context.Background()
	for _, tc := range importCases {
		t.Run(tc.name, func(t *testing.T) {
			r, schemaResp := configured(t, tc, anyPathServer(t, tc.body))
			want := readState(t, tc, tc.body)

			// The plan carries the values the user configured; here every
			// attribute is known and equal to the API object.
			plan := tfsdk.Plan{Schema: schemaResp.Schema, Raw: want.Raw}
			cfg := tfsdk.Config{Schema: schemaResp.Schema, Raw: want.Raw}
			nullState := tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), nil)}

			createResp := resource.CreateResponse{State: nullState}
			r.Create(ctx, resource.CreateRequest{Config: cfg, Plan: plan}, &createResp)
			if createResp.Diagnostics.HasError() {
				t.Fatalf("Create: %v", createResp.Diagnostics)
			}
			updateResp := resource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: want.Raw}}
			r.Update(ctx, resource.UpdateRequest{Config: cfg, Plan: plan, State: stateWithOnlyID(t, schemaResp, tc.id)}, &updateResp)
			if updateResp.Diagnostics.HasError() {
				t.Fatalf("Update: %v", updateResp.Diagnostics)
			}

			if !createResp.State.Raw.Equal(want.Raw) {
				t.Errorf("Create and Read states differ:\ncreate=%s\nread=%s", createResp.State.Raw, want.Raw)
			}
			if !updateResp.State.Raw.Equal(want.Raw) {
				t.Errorf("Update and Read states differ:\nupdate=%s\nread=%s", updateResp.State.Raw, want.Raw)
			}
		})
	}
}

// TestUpdateMirrorsNormalizedAPIResponse covers the case where the PATCH
// returns values different from the planned ones (the server normalizes
// them): the state after Update must be exactly what the API returned.
func TestUpdateMirrorsNormalizedAPIResponse(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		row   string
		patch string
	}{
		{"ipzilon_next_ip_address", `{"id":2204,"subnet_id":65,"address":"10.1.4.10","status":"used","is_azure_reserved":false,"hostname":"normalized-host","description":"normalized"}`},
		{"ipzilon_next_subnet", `{"id":11,"network_id":3,"name":"normalized","cidr":"10.1.4.0/24","description":"normalized","zone_id":30}`},
		{"ipzilon_next_network", `{"id":21,"scope_id":8,"name":"normalized","cidr":"10.2.0.0/22","description":"normalized"}`},
	}
	for _, c := range cases {
		t.Run(c.row, func(t *testing.T) {
			var tc importCase
			for _, ic := range importCases {
				if ic.name == c.row {
					tc = ic
				}
			}
			original := readState(t, tc, tc.body)
			want := readState(t, tc, c.patch)

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodPatch {
					_, _ = w.Write([]byte(c.patch))
					return
				}
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(srv.Close)
			r, schemaResp := configured(t, tc, &client.Client{BaseURL: srv.URL, Token: "test", HTTPClient: http.DefaultClient})

			resp := resource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: original.Raw}}
			r.Update(ctx, resource.UpdateRequest{
				Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: original.Raw},
				Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: original.Raw},
				State:  tfsdk.State{Schema: schemaResp.Schema, Raw: original.Raw},
			}, &resp)
			if resp.Diagnostics.HasError() {
				t.Fatalf("Update: %v", resp.Diagnostics)
			}
			if !resp.State.Raw.Equal(want.Raw) {
				t.Errorf("state after Update does not mirror the API response:\ngot =%s\nwant=%s", resp.State.Raw, want.Raw)
			}
		})
	}
}

// TestReadAfterImportInvalidCIDR checks that a CIDR that cannot be parsed is
// reported instead of leaving prefix_length empty (which would force a
// replacement on the next plan).
func TestReadAfterImportInvalidCIDR(t *testing.T) {
	cases := []struct {
		name        string
		newResource func() resource.Resource
		path, body  string
	}{
		{"ipzilon_next_subnet", resources.NewNextSubnetResource, "/subnets/1", `{"id":1,"network_id":3,"name":"a","cidr":"garbage","description":null}`},
		{"ipzilon_last_subnet", resources.NewLastSubnetResource, "/subnets/1", `{"id":1,"network_id":3,"name":"a","cidr":"garbage","description":null}`},
		{"ipzilon_next_network", resources.NewNextNetworkResource, "/networks/1", `{"id":1,"scope_id":3,"name":"a","cidr":"garbage","description":null}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ic := importCase{name: tc.name, newResource: tc.newResource}
			r, schemaResp := configured(t, ic, newAPIServer(t, tc.path, tc.body))
			resp := resource.ReadResponse{State: stateWithOnlyID(t, schemaResp, 1)}
			r.Read(context.Background(), resource.ReadRequest{State: stateWithOnlyID(t, schemaResp, 1)}, &resp)
			if !resp.Diagnostics.HasError() {
				t.Fatal("Read with an invalid CIDR returned no error")
			}
		})
	}
}

// TestReadNotFoundRemovesFromState keeps the drift detection of Principle I:
// a 404 removes the resource from the state.
func TestReadNotFoundRemovesFromState(t *testing.T) {
	for _, tc := range importCases {
		t.Run(tc.name, func(t *testing.T) {
			r, schemaResp := configured(t, tc, newAPIServer(t, "/nothing-here", "{}"))
			resp := resource.ReadResponse{State: stateWithOnlyID(t, schemaResp, tc.id)}
			r.Read(context.Background(), resource.ReadRequest{State: stateWithOnlyID(t, schemaResp, tc.id)}, &resp)
			if resp.Diagnostics.HasError() {
				t.Fatalf("Read: %v", resp.Diagnostics)
			}
			if !resp.State.Raw.IsNull() {
				t.Error("resource still in state after a 404")
			}
		})
	}
}

// TestReadKeepsHostnameEqualToDescription documents R6: when the API returns
// the same value for hostname and description (migrated IPs such as id 2204)
// the state mirrors it as-is.
func TestReadKeepsHostnameEqualToDescription(t *testing.T) {
	body := `{"id":2204,"subnet_id":65,"address":"10.1.4.10","status":"reserved","is_azure_reserved":false,"hostname":"ilb-events","description":"ilb-events"}`
	for _, tc := range []importCase{
		{name: "ipzilon_next_ip_address", newResource: resources.NewNextIPAddressResource},
		{name: "ipzilon_ip_address", newResource: resources.NewIPAddressResource},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, schemaResp := configured(t, tc, newAPIServer(t, "/ips/2204", body))
			resp := resource.ReadResponse{State: stateWithOnlyID(t, schemaResp, 2204)}
			r.Read(context.Background(), resource.ReadRequest{State: stateWithOnlyID(t, schemaResp, 2204)}, &resp)
			if resp.Diagnostics.HasError() {
				t.Fatalf("Read: %v", resp.Diagnostics)
			}
			assertComplete(t, tc.name, schemaResp, resp.State)
			assertWant(t, tc.name, attrValues(t, resp.State), map[string]any{"hostname": "ilb-events", "description": "ilb-events"})
		})
	}
}

// TestReadAfterImportSubnetZoneIDNull covers subnets outside every zone (and
// IPzilon 3.0.x, which does not return zone_id): zone_id is null, not unknown.
func TestReadAfterImportSubnetZoneIDNull(t *testing.T) {
	bodies := map[string]string{
		"zone_id_null":    `{"id":9,"network_id":6,"name":"sn","cidr":"10.1.0.0/24","description":null,"zone_id":null}`,
		"zone_id_missing": `{"id":9,"network_id":6,"name":"sn","cidr":"10.1.0.0/24","description":null}`,
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			tc := importCase{name: "ipzilon_subnet", newResource: resources.NewSubnetResource, id: 9, path: "/subnets/9", body: body}
			r, schemaResp := configured(t, tc, newAPIServer(t, tc.path, tc.body))
			resp := resource.ReadResponse{State: stateWithOnlyID(t, schemaResp, tc.id)}
			r.Read(context.Background(), resource.ReadRequest{State: stateWithOnlyID(t, schemaResp, tc.id)}, &resp)
			if resp.Diagnostics.HasError() {
				t.Fatalf("Read: %v", resp.Diagnostics)
			}
			assertComplete(t, tc.name, schemaResp, resp.State)
			assertWant(t, tc.name, attrValues(t, resp.State), map[string]any{"zone_id": nil})
		})
	}
}
