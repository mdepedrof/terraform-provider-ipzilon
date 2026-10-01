package resources_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccDataSources_GlobalLookup creates a hub → scope → network → subnet
// chain and finds each object again by its properties only, without the id
// of any parent (IPzilon >= 3.2.0). The data sources are added in a second
// step so that they are read after the objects exist.
func TestAccDataSources_GlobalLookup(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	testAccPreCheck(t)
	name := accName()
	n := accNetworks(t)
	chain := accChain(t, name, "subnet")
	lookups := chain + fmt.Sprintf(`
data "ipzilon_hubs" "acc" {
  address_space = "%[2]s"
  name          = "%[1]s"
}

data "ipzilon_scopes" "acc" {
  kind = "landing_zone"
  name = "%[1]s"
}

data "ipzilon_networks" "acc" {
  cidr = "%[3]s"
  name = "%[1]s"
}

data "ipzilon_subnets" "acc" {
  cidr = "%[4]s"
  name = "%[1]s"
}

data "ipzilon_networks" "missing" {
  name = "%[1]s-missing"
}
`, name, n.space, n.network, n.subnet)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             accCheckDestroy,
		Steps: []resource.TestStep{
			{Config: chain},
			{
				Config: lookups,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.ipzilon_hubs.acc", "items.#", "1"),
					resource.TestCheckResourceAttrPair("data.ipzilon_hubs.acc", "items.0.id", "ipzilon_hub.acc", "id"),
					resource.TestCheckResourceAttr("data.ipzilon_scopes.acc", "items.#", "1"),
					resource.TestCheckResourceAttrPair("data.ipzilon_scopes.acc", "items.0.id", "ipzilon_scope.acc", "id"),
					resource.TestCheckResourceAttr("data.ipzilon_networks.acc", "items.#", "1"),
					resource.TestCheckResourceAttrPair("data.ipzilon_networks.acc", "items.0.id", "ipzilon_network.acc", "id"),
					resource.TestCheckResourceAttr("data.ipzilon_subnets.acc", "items.#", "1"),
					resource.TestCheckResourceAttrPair("data.ipzilon_subnets.acc", "items.0.id", "ipzilon_subnet.acc", "id"),
					resource.TestCheckResourceAttr("data.ipzilon_networks.missing", "items.#", "0"),
				),
			},
		},
	})
}

// TestAccHub_MoveSite moves a hub with a scope, a network and a subnet to
// another site (IPzilon >= 3.2.0): the hub is updated in place keeping its id,
// nothing under it changes and the next plan is empty. It needs a second site
// of the same type as IPZILON_TEST_SITE_ID in IPZILON_TEST_ALT_SITE_ID.
func TestAccHub_MoveSite(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	alt := os.Getenv("IPZILON_TEST_ALT_SITE_ID")
	if alt == "" {
		t.Skip("set IPZILON_TEST_ALT_SITE_ID to a second site of the same type to test moving a hub")
	}
	testAccPreCheck(t)
	name := accName()
	chain := accChain(t, name, "subnet")
	site := `site_id       = ` + os.Getenv("IPZILON_TEST_SITE_ID")
	moved := strings.Replace(chain, site, `site_id       = `+alt, 1)
	if moved == chain {
		t.Fatalf("hub site_id not found in the configuration:\n%s", chain)
	}

	var hubID string
	hubIDIs := func(want *string, capture bool) resource.TestCheckFunc {
		return func(s *terraform.State) error {
			rs, ok := s.RootModule().Resources["ipzilon_hub.acc"]
			if !ok {
				return fmt.Errorf("ipzilon_hub.acc not found in state")
			}
			if capture {
				*want = rs.Primary.ID
				return nil
			}
			if rs.Primary.ID != *want {
				return fmt.Errorf("hub id changed from %s to %s", *want, rs.Primary.ID)
			}
			return nil
		}
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             accCheckDestroy,
		Steps: []resource.TestStep{
			{Config: chain, Check: hubIDIs(&hubID, true)},
			{
				Config: moved,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ipzilon_hub.acc", plancheck.ResourceActionUpdate),
						plancheck.ExpectResourceAction("ipzilon_scope.acc", plancheck.ResourceActionNoop),
						plancheck.ExpectResourceAction("ipzilon_network.acc", plancheck.ResourceActionNoop),
						plancheck.ExpectResourceAction("ipzilon_subnet.acc", plancheck.ResourceActionNoop),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					hubIDIs(&hubID, false),
					resource.TestCheckResourceAttr("ipzilon_hub.acc", "site_id", alt),
				),
			},
			{Config: moved, PlanOnly: true},
		},
	})
}
