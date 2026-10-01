package resources

import (
	"context"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/mdepedrof/terraform-provider-ipzilon/internal/client"
)

// apiCall is one request received by fakeAPI.
type apiCall struct {
	Method string
	Path   string
	Body   map[string]any
}

// apiReply is the canned answer of a route ("METHOD /path").
type apiReply struct {
	Status int
	Body   string
}

// fakeAPI is an IPzilon stand-in that records every request and answers by
// exact route. A request without a route fails the test.
type fakeAPI struct {
	Calls []apiCall
}

func newFakeAPI(t *testing.T, version string, routes map[string]apiReply) (*client.Client, *fakeAPI) {
	t.Helper()
	api := &fakeAPI{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := apiCall{Method: r.Method, Path: r.URL.RequestURI()}
		if raw, _ := io.ReadAll(r.Body); len(raw) > 0 {
			_ = json.Unmarshal(raw, &call.Body)
		}
		api.Calls = append(api.Calls, call)
		reply, ok := routes[r.Method+" "+r.URL.RequestURI()]
		if !ok {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.RequestURI())
			reply = apiReply{Status: http.StatusNotFound, Body: `{"detail":"not found"}`}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(reply.Status)
		_, _ = w.Write([]byte(reply.Body))
	}))
	t.Cleanup(srv.Close)
	return &client.Client{BaseURL: srv.URL, Token: "test", HTTPClient: http.DefaultClient, APIVersion: version}, api
}

// unknown marks an attribute as unknown in objectValue.
type unknownMarker struct{}

var unknown = unknownMarker{}

// objectValue builds a value of the resource schema from Go values (int64,
// string, bool, nil or unknown); attributes not in vals are null.
func objectValue(t *testing.T, schemaResp resource.SchemaResponse, vals map[string]any) tftypes.Value {
	t.Helper()
	objType := schemaResp.Schema.Type().TerraformType(context.Background()).(tftypes.Object)
	out := make(map[string]tftypes.Value, len(objType.AttributeTypes))
	for name, typ := range objType.AttributeTypes {
		switch v := vals[name].(type) {
		case nil:
			out[name] = tftypes.NewValue(typ, nil)
		case unknownMarker:
			out[name] = tftypes.NewValue(typ, tftypes.UnknownValue)
		case int64:
			out[name] = tftypes.NewValue(typ, big.NewFloat(float64(v)))
		case int:
			out[name] = tftypes.NewValue(typ, big.NewFloat(float64(v)))
		default:
			out[name] = tftypes.NewValue(typ, v)
		}
	}
	return tftypes.NewValue(objType, out)
}

// harness wraps a configured resource and builds its requests.
type harness struct {
	t      *testing.T
	r      resource.Resource
	schema resource.SchemaResponse
}

func newHarness(t *testing.T, r resource.Resource, c *client.Client) harness {
	t.Helper()
	var schemaResp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &schemaResp)
	var cfgResp resource.ConfigureResponse
	r.(resource.ResourceWithConfigure).Configure(context.Background(), resource.ConfigureRequest{ProviderData: c}, &cfgResp)
	if cfgResp.Diagnostics.HasError() {
		t.Fatalf("Configure: %v", cfgResp.Diagnostics)
	}
	return harness{t: t, r: r, schema: schemaResp}
}

func (h harness) config(vals map[string]any) tfsdk.Config {
	return tfsdk.Config{Schema: h.schema.Schema, Raw: objectValue(h.t, h.schema, vals)}
}

func (h harness) plan(vals map[string]any) tfsdk.Plan {
	return tfsdk.Plan{Schema: h.schema.Schema, Raw: objectValue(h.t, h.schema, vals)}
}

func (h harness) state(vals map[string]any) tfsdk.State {
	return tfsdk.State{Schema: h.schema.Schema, Raw: objectValue(h.t, h.schema, vals)}
}

func (h harness) emptyState() tfsdk.State {
	objType := h.schema.Schema.Type().TerraformType(context.Background())
	return tfsdk.State{Schema: h.schema.Schema, Raw: tftypes.NewValue(objType, nil)}
}

func (h harness) create(cfg, plan map[string]any) resource.CreateResponse {
	resp := resource.CreateResponse{State: h.emptyState()}
	h.r.Create(context.Background(), resource.CreateRequest{Config: h.config(cfg), Plan: h.plan(plan)}, &resp)
	return resp
}

func (h harness) read(state map[string]any) resource.ReadResponse {
	resp := resource.ReadResponse{State: h.state(state)}
	h.r.Read(context.Background(), resource.ReadRequest{State: h.state(state)}, &resp)
	return resp
}

func (h harness) update(cfg, plan, state map[string]any) resource.UpdateResponse {
	resp := resource.UpdateResponse{State: h.state(state)}
	h.r.Update(context.Background(), resource.UpdateRequest{Config: h.config(cfg), Plan: h.plan(plan), State: h.state(state)}, &resp)
	return resp
}

func (h harness) delete(state map[string]any) resource.DeleteResponse {
	resp := resource.DeleteResponse{State: h.state(state)}
	h.r.Delete(context.Background(), resource.DeleteRequest{State: h.state(state)}, &resp)
	return resp
}

// modifyPlan runs ModifyPlan for a create (state nil) or an update.
func (h harness) modifyPlan(cfg, plan, state map[string]any) resource.ModifyPlanResponse {
	priorState := h.emptyState()
	if state != nil {
		priorState = h.state(state)
	}
	req := resource.ModifyPlanRequest{Config: h.config(cfg), Plan: h.plan(plan), State: priorState}
	resp := resource.ModifyPlanResponse{Plan: h.plan(plan)}
	h.r.(resource.ResourceWithModifyPlan).ModifyPlan(context.Background(), req, &resp)
	return resp
}

// attr reads one attribute of a state or plan as a Go value (int64, string,
// bool, nil or unknown).
func attr(t *testing.T, raw tftypes.Value, name string) any {
	t.Helper()
	vals := map[string]tftypes.Value{}
	if err := raw.As(&vals); err != nil {
		t.Fatalf("not an object: %v", err)
	}
	v, ok := vals[name]
	if !ok {
		t.Fatalf("attribute %q not in object", name)
	}
	if !v.IsKnown() {
		return unknown
	}
	if v.IsNull() {
		return nil
	}
	switch {
	case v.Type().Is(tftypes.Number):
		var f big.Float
		_ = v.As(&f)
		n, _ := f.Int64()
		return n
	case v.Type().Is(tftypes.String):
		var s string
		_ = v.As(&s)
		return s
	case v.Type().Is(tftypes.Bool):
		var b bool
		_ = v.As(&b)
		return b
	}
	t.Fatalf("attribute %q: unsupported type %s", name, v.Type())
	return nil
}
