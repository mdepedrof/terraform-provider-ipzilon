package datasources

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/mdepedrof/terraform-provider-ipzilon/internal/client"
)

// fakeAPI answers by exact request URI and records the URIs requested. A
// request without a route fails the test.
func fakeAPI(t *testing.T, version string, routes map[string]string) (*client.Client, *[]string) {
	t.Helper()
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.RequestURI())
		body, ok := routes[r.URL.RequestURI()]
		if !ok {
			t.Errorf("unexpected request %s", r.URL.RequestURI())
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"detail":"not found"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return &client.Client{BaseURL: srv.URL, Token: "test", HTTPClient: http.DefaultClient, APIVersion: version}, &calls
}

// readDataSource runs Read with a configuration built from Go values (int64,
// string, bool or nil) and returns the response.
func readDataSource(t *testing.T, d datasource.DataSource, c *client.Client, vals map[string]any) datasource.ReadResponse {
	t.Helper()
	ctx := context.Background()
	var schemaResp datasource.SchemaResponse
	d.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
	d.(datasource.DataSourceWithConfigure).Configure(ctx, datasource.ConfigureRequest{ProviderData: c}, &datasource.ConfigureResponse{})

	objType := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	raw := make(map[string]tftypes.Value, len(objType.AttributeTypes))
	for name, typ := range objType.AttributeTypes {
		switch v := vals[name].(type) {
		case nil:
			raw[name] = tftypes.NewValue(typ, nil)
		case int64:
			raw[name] = tftypes.NewValue(typ, big.NewFloat(float64(v)))
		default:
			raw[name] = tftypes.NewValue(typ, v)
		}
	}
	cfg := tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(objType, raw)}
	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(objType, nil)}}
	d.Read(ctx, datasource.ReadRequest{Config: cfg}, &resp)
	return resp
}

// itemsOf returns the "items" list of the state as one map per item.
func itemsOf(t *testing.T, resp datasource.ReadResponse) []map[string]tftypes.Value {
	t.Helper()
	top := map[string]tftypes.Value{}
	if err := resp.State.Raw.As(&top); err != nil {
		t.Fatalf("state: %v", err)
	}
	var list []tftypes.Value
	if err := top["items"].As(&list); err != nil {
		t.Fatalf("items: %v", err)
	}
	out := make([]map[string]tftypes.Value, 0, len(list))
	for _, v := range list {
		m := map[string]tftypes.Value{}
		if err := v.As(&m); err != nil {
			t.Fatalf("item: %v", err)
		}
		out = append(out, m)
	}
	return out
}

func numberOf(t *testing.T, v tftypes.Value) any {
	t.Helper()
	if v.IsNull() {
		return nil
	}
	var f big.Float
	if err := v.As(&f); err != nil {
		t.Fatalf("number: %v", err)
	}
	n, _ := f.Int64()
	return n
}

func zoneJSON(id int64, network int64, name, cidr string) string {
	return fmt.Sprintf(`{"id":%d,"network_id":%d,"name":%q,"cidr":%q,"description":null,"total_ips":256,"used_ips":0,"available_ips":256,"alert_percent":0,"alert_metric":"block_alloc","subnet_count":0}`, id, network, name, cidr)
}

func TestNetworkZonesByID(t *testing.T) {
	c, _ := fakeAPI(t, "3.1.0", map[string]string{"/zones/12": zoneJSON(12, 7, "pooled_zone_1", "10.0.16.0/23")})
	resp := readDataSource(t, NewNetworkZonesDataSource(), c, map[string]any{"id": int64(12)})
	if resp.Diagnostics.HasError() {
		t.Fatalf("Read: %v", resp.Diagnostics)
	}
	items := itemsOf(t, resp)
	if len(items) != 1 || numberOf(t, items[0]["network_id"]) != int64(7) {
		t.Errorf("items = %v", items)
	}
}

// TestNetworkZonesByCIDRAllPages looks a zone up by CIDR only and walks two
// pages (GetAll uses pages of 1000) keeping the API order.
func TestNetworkZonesByCIDRAllPages(t *testing.T) {
	first := make([]string, 0, 1000)
	for i := 0; i < 1000; i++ {
		first = append(first, zoneJSON(int64(i+1), 7, fmt.Sprintf("z%d", i), "10.0.16.0/23"))
	}
	c, calls := fakeAPI(t, "3.1.0", map[string]string{
		"/zones/?cidr=10.0.16.0%2F23&limit=1000&offset=0":    `{"items":[` + strings.Join(first, ",") + `],"total":1001}`,
		"/zones/?cidr=10.0.16.0%2F23&limit=1000&offset=1000": `{"items":[` + zoneJSON(1001, 8, "last", "10.0.16.0/23") + `],"total":1001}`,
	})
	resp := readDataSource(t, NewNetworkZonesDataSource(), c, map[string]any{"cidr": "10.0.16.0/23"})
	if resp.Diagnostics.HasError() {
		t.Fatalf("Read: %v", resp.Diagnostics)
	}
	items := itemsOf(t, resp)
	if len(items) != 1001 || numberOf(t, items[1000]["id"]) != int64(1001) {
		t.Errorf("got %d items, last id %v", len(items), numberOf(t, items[len(items)-1]["id"]))
	}
	if len(*calls) != 2 {
		t.Errorf("calls = %v, want 2 pages", *calls)
	}
}

func TestNetworkZonesByNameAndNetwork(t *testing.T) {
	c, _ := fakeAPI(t, "3.1.0", map[string]string{
		"/zones/?name=z&network_id=7&limit=1000&offset=0": `{"items":[` + zoneJSON(12, 7, "z", "10.0.16.0/23") + `],"total":1}`,
	})
	resp := readDataSource(t, NewNetworkZonesDataSource(), c, map[string]any{"name": "z", "network_id": int64(7)})
	if resp.Diagnostics.HasError() {
		t.Fatalf("Read: %v", resp.Diagnostics)
	}
	if items := itemsOf(t, resp); len(items) != 1 {
		t.Errorf("items = %v", items)
	}
}

func TestNetworkZonesEmptyIsNotAnError(t *testing.T) {
	c, _ := fakeAPI(t, "3.1.0", map[string]string{
		"/zones/?name=missing&limit=1000&offset=0": `{"items":[],"total":0}`,
	})
	resp := readDataSource(t, NewNetworkZonesDataSource(), c, map[string]any{"name": "missing"})
	if resp.Diagnostics.HasError() {
		t.Fatalf("Read: %v", resp.Diagnostics)
	}
	if items := itemsOf(t, resp); len(items) != 0 {
		t.Errorf("items = %v, want empty list", items)
	}
}

func TestNetworkZonesErrors(t *testing.T) {
	cases := map[string]struct {
		version string
		cfg     map[string]any
		want    string
	}{
		"id with name":    {"3.1.0", map[string]any{"id": int64(12), "name": "z"}, "not both"},
		"old IPzilon":     {"3.0.1", map[string]any{"cidr": "10.0.16.0/23"}, "requires IPzilon >= 3.1.0"},
		"old IPzilon, id": {"3.0.1", map[string]any{"id": int64(12)}, "requires IPzilon >= 3.1.0"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c, calls := fakeAPI(t, tc.version, nil)
			resp := readDataSource(t, NewNetworkZonesDataSource(), c, tc.cfg)
			if !resp.Diagnostics.HasError() || !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), tc.want) {
				t.Errorf("diagnostics = %v, want %q", resp.Diagnostics, tc.want)
			}
			if len(*calls) != 0 {
				t.Errorf("no request expected, got %v", *calls)
			}
		})
	}
}
