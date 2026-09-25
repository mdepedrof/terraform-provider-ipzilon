package resources

import "testing"

func TestCidrPrefixLength(t *testing.T) {
	cases := []struct {
		name    string
		cidr    string
		want    int64
		wantErr bool
	}{
		{name: "/24", cidr: "10.1.4.0/24", want: 24},
		{name: "/16", cidr: "10.0.0.0/16", want: 16},
		{name: "/29 non-aligned host bits ignored by ParseCIDR mask", cidr: "10.1.4.16/29", want: 29},
		{name: "invalid cidr", cidr: "not-a-cidr", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := cidrPrefixLength(tc.cidr)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("cidrPrefixLength(%q) expected error, got nil", tc.cidr)
				}
				return
			}
			if err != nil {
				t.Fatalf("cidrPrefixLength(%q) unexpected error: %v", tc.cidr, err)
			}
			if got != tc.want {
				t.Errorf("cidrPrefixLength(%q) = %d, want %d", tc.cidr, got, tc.want)
			}
		})
	}
}
