// Copyright IBM Corp. 2026 All Rights Reserved.
// Licensed under the Mozilla Public License v2.0

package logsrouting_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"

	acc "github.com/IBM-Cloud/terraform-provider-ibm/ibm/acctest"
)

func TestAccIBMLogsRouterServiceTenantBasic(t *testing.T) {
	parent := fmt.Sprintf("tf-parent-%d", acctest.RandIntRange(10, 100))
	name := fmt.Sprintf("tf-svc-%d", acctest.RandIntRange(10, 100))
	nameUpdate := fmt.Sprintf("tf-svc-%d", acctest.RandIntRange(10, 100))
	crn := "crn:v1:bluemix:public:logs:eu-de:a/3516b8fa0a174a71899f5affa4f18d78:3517d2ed-9429-af34-ad52-34278391cbc8::"
	host := "3517d2ed-9429-af34-ad52-34278391cbc8.ingress.eu-de.logs.cloud.ibm.com"
	res := "ibm_logs_router_service_tenant.service_tenant"

	resource.Test(t, resource.TestCase{
		PreCheck:  func() { acc.TestAccPreCheck(t) },
		Providers: acc.TestAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccIBMLogsRouterServiceTenantConfig(parent, name, crn, host),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "name", name),
					resource.TestCheckResourceAttrSet(res, "crn"),
					resource.TestCheckResourceAttrSet(res, "etag"),
					resource.TestCheckResourceAttr(res, "targets.0.parameters.0.port", "3443"),
				),
			},
			{
				Config: testAccIBMLogsRouterServiceTenantConfig(parent, nameUpdate, crn, host),
				Check:  resource.TestCheckResourceAttr(res, "name", nameUpdate),
			},
			{
				ResourceName:            res,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"account_level_tenant_id", "service_name"},
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources[res]
					if !ok {
						return "", fmt.Errorf("Not found: %s", res)
					}
					return fmt.Sprintf("%s/%s", rs.Primary.ID, rs.Primary.Attributes["region"]), nil
				},
			},
		},
	})
}

func testAccIBMLogsRouterServiceTenantConfig(parent, name, crn, host string) string {
	return fmt.Sprintf(`
resource "ibm_logs_router_tenant" "parent" {
  name   = "%[1]s"
  region = "eu-de"
  targets {
    log_sink_crn = "%[3]s"
    name         = "parent-target"
    parameters {
      host = "%[4]s"
      port = 443
    }
  }
}

resource "ibm_logs_router_service_tenant" "service_tenant" {
  account_level_tenant_id = ibm_logs_router_tenant.parent.id
  service_name            = "tf-acc-service"
  name                    = "%[2]s"
  region                  = "eu-de"
  targets {
    log_sink_crn = "%[3]s"
    name         = "service-target"
    parameters {
      host = "%[4]s"
    }
  }
}
`, parent, name, crn, host)
}
