package resources

import (
	"errors"
	"fmt"

	"github.com/mdepedrof/terraform-provider-ipzilon/internal/client"
)

// allocationErrorDiag builds a specific diagnostic when IPzilon stops looking
// for a free block because the address space is too fragmented (409 "Search
// truncated"), suggesting the resource that takes an explicit CIDR instead.
// ok is false for any other error, which callers report as before.
func allocationErrorDiag(err error, prefixLength int64, explicitResource string) (summary, detail string, ok bool) {
	var apiErr *client.APIError
	if !client.IsSearchTruncated(err) || !errors.As(err, &apiErr) {
		return "", "", false
	}
	return "Address space too fragmented", fmt.Sprintf(
		"IPzilon stopped searching for a free /%d block because the address space is too fragmented. "+
			"Declare an explicit CIDR with %s instead. API: %s",
		prefixLength, explicitResource, apiErr.Message,
	), true
}
