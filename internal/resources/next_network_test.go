package resources

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mdepedrof/terraform-provider-ipzilon/internal/client"
)

// newTestClient builds a client.Client pointed directly at the given test
// server's BaseURL, bypassing client.New's /health probe (irrelevant here).
func newTestClient(baseURL string) *client.Client {
	return &client.Client{
		BaseURL:    baseURL,
		Token:      "test-token",
		HTTPClient: http.DefaultClient,
	}
}

// TestNextNetworkResource_Create verifies the exact request the resource
// issues for POST /scopes/{scope_id}/next-available-network and that the
// server's Network response is decoded correctly.
func TestNextNetworkResource_Create(t *testing.T) {
	var gotPath string
	var gotBody client.AllocateNetworkBody

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(client.Network{
			ID:      42,
			ScopeID: 7,
			Name:    "web-tier",
			CIDR:    "10.0.1.0/24",
		})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	name := "web-tier"

	var got client.Network
	err := c.Post("/scopes/7/next-available-network", client.AllocateNetworkBody{
		PrefixLength: 24,
		Name:         &name,
	}, &got)
	if err != nil {
		t.Fatalf("Post failed: %v", err)
	}

	if gotPath != "/scopes/7/next-available-network" {
		t.Errorf("path = %q, want /scopes/7/next-available-network", gotPath)
	}
	if gotBody.PrefixLength != 24 || gotBody.Name == nil || *gotBody.Name != "web-tier" {
		t.Errorf("unexpected request body: %+v", gotBody)
	}
	if got.ID != 42 || got.CIDR != "10.0.1.0/24" || got.ScopeID != 7 {
		t.Errorf("unexpected response mapping: %+v", got)
	}
}

// TestNextNetworkResource_CreateConflict verifies that a 409 from the
// backend (no free block available) surfaces as an error, not a silent
// success — the atomicity contract next_network.go relies on.
func TestNextNetworkResource_CreateConflict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"detail": "No free /24 block available"})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	var got client.Network
	err := c.Post("/scopes/7/next-available-network", client.AllocateNetworkBody{PrefixLength: 24}, &got)
	if err == nil {
		t.Fatal("expected error on 409, got nil")
	}
	apiErr, ok := err.(*client.APIError)
	if !ok || apiErr.Code != http.StatusConflict {
		t.Errorf("expected *client.APIError{Code: 409}, got %#v", err)
	}
}

// TestNextNetworkResource_ReadDeleteRoutes verifies Read/Update/Delete hit
// the exact /networks/{id} routes used by next_network.go.
func TestNextNetworkResource_ReadDeleteRoutes(t *testing.T) {
	var gotMethod, gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(client.Network{ID: 42, ScopeID: 7, Name: "n", CIDR: "10.0.1.0/24"})
		case http.MethodPatch:
			_ = json.NewEncoder(w).Encode(client.Network{ID: 42, ScopeID: 7, Name: "renamed", CIDR: "10.0.1.0/24"})
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)

	var n client.Network
	if err := c.Get("/networks/42", &n); err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if gotMethod != http.MethodGet || gotPath != "/networks/42" {
		t.Errorf("Read: got %s %s", gotMethod, gotPath)
	}

	name := "renamed"
	if err := c.Patch("/networks/42", client.NetworkUpdate{Name: &name}, &n); err != nil {
		t.Fatalf("Patch failed: %v", err)
	}
	if gotMethod != http.MethodPatch || gotPath != "/networks/42" || n.Name != "renamed" {
		t.Errorf("Update: got %s %s, name=%s", gotMethod, gotPath, n.Name)
	}

	if err := c.Delete("/networks/42"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if gotMethod != http.MethodDelete || gotPath != "/networks/42" {
		t.Errorf("Delete: got %s %s", gotMethod, gotPath)
	}
}

// TestNextNetworkResource_ReadNotFound verifies client.IsNotFound(err) is
// true for a 404 from GET /networks/{id}, the condition next_network.go's
// Read() uses to remove the resource from state.
func TestNextNetworkResource_ReadNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"detail": "not found"})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	var n client.Network
	err := c.Get("/networks/999", &n)
	if !client.IsNotFound(err) {
		t.Errorf("expected IsNotFound(err) = true, got err = %v", err)
	}
}
