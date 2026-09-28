package resources

import (
	"errors"
	"strings"
	"testing"

	"github.com/mdepedrof/terraform-provider-ipzilon/internal/client"
)

func TestAllocationErrorDiag(t *testing.T) {
	truncated := &client.APIError{Code: 409, Message: "Search truncated after 4096 steps without finding a free /28 block in 10.0.0.0/12: the space is too fragmented; request a specific CIDR instead"}

	summary, detail, ok := allocationErrorDiag(truncated, 28, "ipzilon_subnet")
	if !ok || summary != "Address space too fragmented" {
		t.Fatalf("got %q, ok=%v; want Address space too fragmented", summary, ok)
	}
	for _, want := range []string{"/28", "ipzilon_subnet", truncated.Message} {
		if !strings.Contains(detail, want) {
			t.Errorf("detail %q does not contain %q", detail, want)
		}
	}

	for _, err := range []error{
		&client.APIError{Code: 409, Message: "No free /24 block available in 10.0.0.0/16"},
		&client.APIError{Code: 404, Message: "Network not found"},
		errors.New("do request: connection refused"),
	} {
		if _, _, ok := allocationErrorDiag(err, 24, "ipzilon_subnet"); ok {
			t.Errorf("allocationErrorDiag(%v) ok = true, want false", err)
		}
	}
}
