package resources_test

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"

	"github.com/mdepedrof/terraform-provider-ipzilon/internal/client"
	"github.com/mdepedrof/terraform-provider-ipzilon/internal/provider"
)

// Acceptance tests: run against a real IPzilon (>= 3.0.0) with
//
//	TF_ACC=1 IPZILON_API_URL=... IPZILON_TOKEN=... \
//	IPZILON_TEST_SITE_ID=<existing site id> \
//	IPZILON_TEST_ADDRESS_SPACE=<free IPv4 /16, e.g. 10.250.0.0/16> make testacc
//
// Each test creates a hub → scope → network (→ subnet) chain plus the resource
// under test with the prefix tf-acc-, then:
//  1. creates it,
//  2. imports it by id and verifies that the imported state equals the state
//     left by Create (this is what fails when Read forgets an attribute, and
//     what makes the plan after an import empty),
//  3. plans again with the same configuration and expects no changes.
//
// The framework destroys everything at the end and CheckDestroy confirms with
// the API that nothing is left behind.

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"ipzilon": providerserver.NewProtocol6WithError(provider.New("test")()),
}

func testAccPreCheck(t *testing.T) {
	t.Helper()
	for _, env := range []string{"IPZILON_API_URL", "IPZILON_TOKEN", "IPZILON_TEST_SITE_ID", "IPZILON_TEST_ADDRESS_SPACE"} {
		if os.Getenv(env) == "" {
			t.Fatalf("%s must be set for acceptance tests", env)
		}
	}
	if _, err := strconv.ParseInt(os.Getenv("IPZILON_TEST_SITE_ID"), 10, 64); err != nil {
		t.Fatalf("IPZILON_TEST_SITE_ID must be a number: %v", err)
	}
	if !strings.HasSuffix(os.Getenv("IPZILON_TEST_ADDRESS_SPACE"), ".0.0/16") {
		t.Fatal("IPZILON_TEST_ADDRESS_SPACE must be an IPv4 /16 like 10.250.0.0/16")
	}
}

// accNet holds the CIDRs the acceptance configs carve out of the /16 test space.
type accNet struct {
	space, network, subnet, ip string
}

func accNetworks(t *testing.T) accNet {
	t.Helper()
	base := strings.TrimSuffix(os.Getenv("IPZILON_TEST_ADDRESS_SPACE"), ".0.0/16") // e.g. 10.250
	return accNet{
		space:   base + ".0.0/16",
		network: base + ".0.0/20",
		subnet:  base + ".0.0/24",
		ip:      base + ".0.10",
	}
}

// accChain returns the HCL of the parents needed by the resource under test.
// Levels: "hub", "scope", "network", "subnet" (each includes the previous ones).
func accChain(t *testing.T, name, upto string) string {
	t.Helper()
	n := accNetworks(t)
	levels := []string{"hub", "scope", "network", "subnet"}
	var b strings.Builder
	for _, lvl := range levels {
		switch lvl {
		case "hub":
			fmt.Fprintf(&b, `
resource "ipzilon_hub" "acc" {
  site_id       = %s
  name          = "%s"
  address_space = "%s"
}
`, os.Getenv("IPZILON_TEST_SITE_ID"), name, n.space)
		case "scope":
			fmt.Fprintf(&b, `
resource "ipzilon_scope" "acc" {
  hub_id = ipzilon_hub.acc.id
  name   = "%s"
  kind   = "landing_zone"
  cidr   = "%s"
}
`, name, n.space)
		case "network":
			fmt.Fprintf(&b, `
resource "ipzilon_network" "acc" {
  scope_id = ipzilon_scope.acc.id
  name     = "%s"
  cidr     = "%s"
}
`, name, n.network)
		case "subnet":
			fmt.Fprintf(&b, `
resource "ipzilon_subnet" "acc" {
  network_id = ipzilon_network.acc.id
  name       = "%s"
  cidr       = "%s"
}
`, name, n.subnet)
		}
		if lvl == upto {
			break
		}
	}
	return b.String()
}

// accCheckDestroy confirms with the API that every managed object is gone.
func accCheckDestroy(s *terraform.State) error {
	paths := map[string]string{
		"ipzilon_hub":             "/hubs/",
		"ipzilon_scope":           "/scopes/",
		"ipzilon_network":         "/networks/",
		"ipzilon_next_network":    "/networks/",
		"ipzilon_subnet":          "/subnets/",
		"ipzilon_next_subnet":     "/subnets/",
		"ipzilon_last_subnet":     "/subnets/",
		"ipzilon_ip_address":      "/ips/",
		"ipzilon_next_ip_address": "/ips/",
	}
	c := client.New(os.Getenv("IPZILON_API_URL"), os.Getenv("IPZILON_TOKEN"))
	for _, rs := range s.RootModule().Resources {
		prefix, ok := paths[rs.Type]
		if !ok {
			continue
		}
		var out map[string]any
		err := c.Get(context.Background(), prefix+rs.Primary.ID, &out)
		if err == nil {
			return fmt.Errorf("%s %s still exists in IPzilon", rs.Type, rs.Primary.ID)
		}
		if !client.IsNotFound(err) {
			return fmt.Errorf("checking %s %s: %w", rs.Type, rs.Primary.ID, err)
		}
	}
	return nil
}

// accImportTest runs the create → import (verify) → empty plan cycle for the
// resource "<resourceType>.acc" whose HCL is resourceHCL, on top of parents
// (the HCL of the objects it depends on, "" if none).
//
// Steps 1–3 create it, import it in a scratch state and compare it with the
// state Create left, and plan again. Steps 4–5 are the real migration flow:
// the resource leaves the state without being destroyed (removed block) and
// is imported back with an import block; the plan that the framework runs
// after that apply must be empty, which is exactly what failed when Read left
// an attribute empty (e.g. subnet_id forcing a replacement).
func accImportTest(t *testing.T, resourceType, parents, resourceHCL string) {
	t.Helper()
	testAccPreCheck(t)

	address := resourceType + ".acc"
	cfg := parents + resourceHCL
	var id string
	vars := map[string]config.Variable{"import_id": config.StringVariable("")}
	importConfig := `
variable "import_id" {
  type = string
}

import {
  to = ` + address + `
  id = var.import_id
}
` + parents + resourceHCL

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_7_0)},
		CheckDestroy:             accCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: func(s *terraform.State) error {
					rs, ok := s.RootModule().Resources[address]
					if !ok {
						return fmt.Errorf("%s not found in state", address)
					}
					id = rs.Primary.ID
					return nil
				},
			},
			{
				Config:            cfg,
				ResourceName:      address,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{Config: cfg, PlanOnly: true},
			{
				Config: parents + `
removed {
  from = ` + address + `
  lifecycle {
    destroy = false
  }
}
`,
			},
			{
				PreConfig:       func() { vars["import_id"] = config.StringVariable(id) },
				Config:          importConfig,
				ConfigVariables: vars,
			},
		},
	})
}

func accName() string { return "tf-acc-" + strings.ToLower(acctest.RandString(8)) }

func TestAccImport_Hub(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	testAccPreCheck(t)
	name := accName()
	n := accNetworks(t)
	accImportTest(t, "ipzilon_hub", "", fmt.Sprintf(`
resource "ipzilon_hub" "acc" {
  site_id       = %s
  name          = "%s"
  address_space = "%s"
}
`, os.Getenv("IPZILON_TEST_SITE_ID"), name, n.space))
}

func TestAccImport_Scope(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	testAccPreCheck(t)
	name := accName()
	accImportTest(t, "ipzilon_scope", accChain(t, name, "hub"), fmt.Sprintf(`
resource "ipzilon_scope" "acc" {
  hub_id = ipzilon_hub.acc.id
  name   = "%s"
  kind   = "landing_zone"
  cidr   = "%s"
}
`, name, accNetworks(t).space))
}

func TestAccImport_Network(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	testAccPreCheck(t)
	name := accName()
	accImportTest(t, "ipzilon_network", accChain(t, name, "scope"), fmt.Sprintf(`
resource "ipzilon_network" "acc" {
  scope_id = ipzilon_scope.acc.id
  name     = "%s"
  cidr     = "%s"
}
`, name, accNetworks(t).network))
}

func TestAccImport_Subnet(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	testAccPreCheck(t)
	name := accName()
	accImportTest(t, "ipzilon_subnet", accChain(t, name, "network"), fmt.Sprintf(`
resource "ipzilon_subnet" "acc" {
  network_id = ipzilon_network.acc.id
  name       = "%s"
  cidr       = "%s"
}
`, name, accNetworks(t).subnet))
}

func TestAccImport_IPAddress(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	testAccPreCheck(t)
	name := accName()
	accImportTest(t, "ipzilon_ip_address", accChain(t, name, "subnet"), fmt.Sprintf(`
resource "ipzilon_ip_address" "acc" {
  subnet_id = ipzilon_subnet.acc.id
  address   = "%s"
  status    = "reserved"
  hostname  = "%s"
}
`, accNetworks(t).ip, name))
}

func TestAccImport_NextIPAddress(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	testAccPreCheck(t)
	name := accName()
	accImportTest(t, "ipzilon_next_ip_address", accChain(t, name, "subnet"), fmt.Sprintf(`
resource "ipzilon_next_ip_address" "acc" {
  subnet_id = ipzilon_subnet.acc.id
  hostname  = "%s"
}
`, name))
}

func TestAccImport_NextSubnet(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	testAccPreCheck(t)
	name := accName()
	accImportTest(t, "ipzilon_next_subnet", accChain(t, name, "network"), fmt.Sprintf(`
resource "ipzilon_next_subnet" "acc" {
  network_id    = ipzilon_network.acc.id
  prefix_length = 26
  name          = "%s"
}
`, name))
}

func TestAccImport_LastSubnet(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	testAccPreCheck(t)
	name := accName()
	accImportTest(t, "ipzilon_last_subnet", accChain(t, name, "network"), fmt.Sprintf(`
resource "ipzilon_last_subnet" "acc" {
  network_id    = ipzilon_network.acc.id
  prefix_length = 26
  name          = "%s"
}
`, name))
}

func TestAccImport_NextNetwork(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	testAccPreCheck(t)
	name := accName()
	accImportTest(t, "ipzilon_next_network", accChain(t, name, "scope"), fmt.Sprintf(`
resource "ipzilon_next_network" "acc" {
  scope_id      = ipzilon_scope.acc.id
  prefix_length = 24
  name          = "%s"
}
`, name))
}
