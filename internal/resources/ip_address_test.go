package resources

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/mdepedrof/terraform-provider-ipzilon/internal/client"
)

// fixedServer answers every request with code/body and records the last
// method, path and body received.
func fixedServer(t *testing.T, code int, body string) (*httptest.Server, *string, *string, *string) {
	t.Helper()
	var gotMethod, gotPath, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(code)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv, &gotMethod, &gotPath, &gotBody
}

func TestRegisterIP(t *testing.T) {
	hostname := "web"
	body := client.IPAddressRegister{Address: "10.0.1.17", Status: "used", Hostname: &hostname}

	cases := []struct {
		name        string
		code        int
		respBody    string
		wantSummary string
		wantDetail  string
	}{
		{name: "created", code: 201, respBody: `{"id":9,"subnet_id":5,"address":"10.0.1.17","status":"used","hostname":"web"}`},
		{name: "in use", code: 409, respBody: `{"detail":"IP 10.0.1.17 is already in use in this subnet"}`,
			wantSummary: "IP address in use", wantDetail: "Address 10.0.1.17 is already in use in subnet 5."},
		{name: "outside subnet", code: 400, respBody: `{"detail":"IP 10.0.1.17 is not within subnet 10.0.2.0/24"}`,
			wantSummary: "IP address outside subnet", wantDetail: "Address 10.0.1.17 is not within subnet 5."},
		{name: "other 400", code: 400, respBody: `{"detail":"something else"}`,
			wantSummary: "Create IP failed", wantDetail: "API error 400: something else"},
		{name: "subnet not found", code: 404, respBody: `{"detail":"Subnet not found"}`,
			wantSummary: "Subnet not found", wantDetail: "Subnet 5 does not exist."},
		{name: "null id", code: 201, respBody: `{"id":null,"subnet_id":5,"address":"10.0.1.17","status":"available"}`,
			wantSummary: "Create IP failed", wantDetail: "Unexpected response: IP address without id"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, method, gotPath, gotBody := fixedServer(t, tc.code, tc.respBody)
			c := newTestClient(srv.URL)

			ip, diags := registerIP(context.Background(), c, 5, body)

			if *method != http.MethodPost || *gotPath != "/subnets/5/ips" {
				t.Errorf("request = %s %s, want POST /subnets/5/ips", *method, *gotPath)
			}
			var sent map[string]any
			_ = json.Unmarshal([]byte(*gotBody), &sent)
			if _, hasDescription := sent["description"]; hasDescription || sent["status"] != "used" || sent["hostname"] != "web" {
				t.Errorf("body = %s", *gotBody)
			}

			if tc.wantSummary == "" {
				if diags.HasError() {
					t.Fatalf("unexpected diagnostics: %v", diags)
				}
				if ip.ID == nil || *ip.ID != 9 {
					t.Errorf("ip.ID = %v, want 9", ip.ID)
				}
				return
			}
			if !diags.HasError() {
				t.Fatal("expected an error diagnostic")
			}
			if diags[0].Summary() != tc.wantSummary || diags[0].Detail() != tc.wantDetail {
				t.Errorf("diag = %q / %q, want %q / %q", diags[0].Summary(), diags[0].Detail(), tc.wantSummary, tc.wantDetail)
			}
		})
	}
}

func TestReleaseIP(t *testing.T) {
	for _, tc := range []struct {
		code    int
		wantErr bool
	}{{204, false}, {404, false}, {403, true}} {
		srv, method, gotPath, _ := fixedServer(t, tc.code, `{"detail":"x"}`)
		err := releaseIP(context.Background(), newTestClient(srv.URL), 7)
		if *method != http.MethodDelete || *gotPath != "/ips/7" {
			t.Errorf("request = %s %s, want DELETE /ips/7", *method, *gotPath)
		}
		if (err != nil) != tc.wantErr {
			t.Errorf("code %d: err = %v, wantErr %v", tc.code, err, tc.wantErr)
		}
	}
}

// TestIPStatusValidation checks that both address resources only accept
// used/reserved: a free address has no record since IPzilon 3.0.
func TestIPStatusValidation(t *testing.T) {
	for name, r := range map[string]resource.Resource{
		"ipzilon_ip_address":      NewIPAddressResource(),
		"ipzilon_next_ip_address": NewNextIPAddressResource(),
	} {
		resp := &resource.SchemaResponse{}
		r.Schema(context.Background(), resource.SchemaRequest{}, resp)
		attr := resp.Schema.Attributes["status"].(schema.StringAttribute)

		for value, wantErr := range map[string]bool{"used": false, "reserved": false, "available": true} {
			vresp := &validator.StringResponse{}
			for _, v := range attr.Validators {
				v.ValidateString(context.Background(), validator.StringRequest{
					Path:        path.Root("status"),
					ConfigValue: types.StringValue(value),
				}, vresp)
			}
			if vresp.Diagnostics.HasError() != wantErr {
				t.Errorf("%s status=%q: error = %v, want %v", name, value, vresp.Diagnostics.HasError(), wantErr)
			}
		}
	}
}

// TestIPAddressUpdateBody checks that an unset status (unknown in the plan) is
// not sent: the API rejects status "" with a 422.
func TestIPAddressUpdateBody(t *testing.T) {
	for _, tc := range []struct {
		status types.String
		want   string
	}{
		{types.StringUnknown(), `{"hostname":"gw","description":null}`},
		{types.StringValue("reserved"), `{"status":"reserved","hostname":"gw","description":null}`},
	} {
		b, err := json.Marshal(ipAddressUpdateBody(ipAddressModel{
			Status:      tc.status,
			Hostname:    types.StringValue("gw"),
			Description: types.StringUnknown(),
		}))
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != tc.want {
			t.Errorf("body = %s, want %s", b, tc.want)
		}
	}
}
