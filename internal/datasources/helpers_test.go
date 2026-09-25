package datasources

import "testing"

func strp(s string) *string { return &s }

func TestHubScopesURL(t *testing.T) {
	cases := []struct {
		name      string
		hubID     int64
		hasParent bool
		parentID  int64
		rootOnly  bool
		cidr      *string
		scopeName *string
		want      string
	}{
		{
			name:  "no filters",
			hubID: 1,
			want:  "/hubs/1/scopes",
		},
		{
			name:      "parent_id only",
			hubID:     1,
			hasParent: true,
			parentID:  5,
			want:      "/hubs/1/scopes?parent_id=5",
		},
		{
			name:     "root_only only",
			hubID:    1,
			rootOnly: true,
			want:     "/hubs/1/scopes?root_only=true",
		},
		{
			name:  "cidr only",
			hubID: 1,
			cidr:  strp("10.0.0.0/24"),
			want:  "/hubs/1/scopes?cidr=10.0.0.0%2F24",
		},
		{
			name:      "name only",
			hubID:     1,
			scopeName: strp("prod"),
			want:      "/hubs/1/scopes?name=prod",
		},
		{
			name:      "cidr combined with parent_id",
			hubID:     1,
			hasParent: true,
			parentID:  5,
			cidr:      strp("10.0.0.0/24"),
			want:      "/hubs/1/scopes?cidr=10.0.0.0%2F24&parent_id=5",
		},
		{
			name:      "cidr and name combined with root_only",
			hubID:     2,
			rootOnly:  true,
			cidr:      strp("10.0.0.0/24"),
			scopeName: strp("prod"),
			want:      "/hubs/2/scopes?cidr=10.0.0.0%2F24&name=prod&root_only=true",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := hubScopesURL(tc.hubID, tc.hasParent, tc.parentID, tc.rootOnly, tc.cidr, tc.scopeName)
			if got != tc.want {
				t.Errorf("hubScopesURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestScopeNetworksURL(t *testing.T) {
	cases := []struct {
		name        string
		scopeID     int64
		cidr        *string
		networkName *string
		want        string
	}{
		{
			name:    "no filters",
			scopeID: 3,
			want:    "/scopes/3/networks",
		},
		{
			name:    "cidr only",
			scopeID: 3,
			cidr:    strp("10.0.1.0/24"),
			want:    "/scopes/3/networks?cidr=10.0.1.0%2F24",
		},
		{
			name:        "name only",
			scopeID:     3,
			networkName: strp("web-tier"),
			want:        "/scopes/3/networks?name=web-tier",
		},
		{
			name:        "cidr and name combined",
			scopeID:     3,
			cidr:        strp("10.0.1.0/24"),
			networkName: strp("web-tier"),
			want:        "/scopes/3/networks?cidr=10.0.1.0%2F24&name=web-tier",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := scopeNetworksURL(tc.scopeID, tc.cidr, tc.networkName)
			if got != tc.want {
				t.Errorf("scopeNetworksURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestHubNetworksURL(t *testing.T) {
	cases := []struct {
		name        string
		hubID       int64
		cidr        *string
		networkName *string
		want        string
	}{
		{
			name:  "no filters",
			hubID: 5,
			want:  "/hubs/5/networks",
		},
		{
			name:  "cidr only",
			hubID: 5,
			cidr:  strp("10.0.1.0/24"),
			want:  "/hubs/5/networks?cidr=10.0.1.0%2F24",
		},
		{
			name:        "name only",
			hubID:       5,
			networkName: strp("web-tier"),
			want:        "/hubs/5/networks?name=web-tier",
		},
		{
			name:        "cidr and name combined",
			hubID:       5,
			cidr:        strp("10.0.1.0/24"),
			networkName: strp("web-tier"),
			want:        "/hubs/5/networks?cidr=10.0.1.0%2F24&name=web-tier",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := hubNetworksURL(tc.hubID, tc.cidr, tc.networkName)
			if got != tc.want {
				t.Errorf("hubNetworksURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSitesURL(t *testing.T) {
	cases := []struct {
		name     string
		siteName *string
		want     string
	}{
		{
			name: "no filters",
			want: "/sites/",
		},
		{
			name:     "name only",
			siteName: strp("hq"),
			want:     "/sites/?name=hq",
		},
		{
			name:     "name with spaces",
			siteName: strp("main office"),
			want:     "/sites/?name=main+office",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sitesURL(tc.siteName)
			if got != tc.want {
				t.Errorf("sitesURL() = %q, want %q", got, tc.want)
			}
		})
	}
}
func TestSiteHubsURL(t *testing.T) {
	cases := []struct {
		name         string
		siteID       int64
		addressSpace *string
		hubName      *string
		want         string
	}{
		{
			name:   "no filters",
			siteID: 1,
			want:   "/sites/1/hubs",
		},
		{
			name:         "address_space only",
			siteID:       1,
			addressSpace: strp("10.0.0.0/16"),
			want:         "/sites/1/hubs?address_space=10.0.0.0%2F16",
		},
		{
			name:    "name only",
			siteID:  1,
			hubName: strp("core"),
			want:    "/sites/1/hubs?name=core",
		},
		{
			name:         "address_space and name combined",
			siteID:       2,
			addressSpace: strp("10.1.0.0/16"),
			hubName:      strp("edge"),
			want:         "/sites/2/hubs?address_space=10.1.0.0%2F16&name=edge",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := siteHubsURL(tc.siteID, tc.addressSpace, tc.hubName)
			if got != tc.want {
				t.Errorf("siteHubsURL() = %q, want %q", got, tc.want)
			}
		})
	}
}
