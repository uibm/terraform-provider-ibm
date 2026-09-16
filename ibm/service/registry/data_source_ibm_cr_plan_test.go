// Copyright IBM Corp. 2026 All Rights Reserved.
// Licensed under the Mozilla Public License v2.0

package registry_test

import (
	"fmt"
	"strings"
	"testing"

	acc "github.com/IBM-Cloud/terraform-provider-ibm/ibm/acctest"
	"github.com/IBM-Cloud/terraform-provider-ibm/ibm/service/registry"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccIBMCrPlanDataSourceBasic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:  func() { acc.TestAccPreCheck(t) },
		Providers: acc.TestAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccCheckIBMCrPlanDataSourceConfigBasic(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.ibm_cr_plan.cr_plan", "id"),
					resource.TestCheckResourceAttrSet("data.ibm_cr_plan.cr_plan", "registry"),
					resource.TestCheckResourceAttrPair("data.ibm_cr_plan.cr_plan", "id", "data.ibm_cr_plan.cr_plan", "registry"),
					testAccCheckIBMCrPlanDataSourceKnownPlan("data.ibm_cr_plan.cr_plan"),
				),
			},
		},
	})
}

func testAccCheckIBMCrPlanDataSourceConfigBasic() string {
	return `
		data "ibm_cr_plan" "cr_plan" {
		}
	`
}

func testAccCheckIBMCrPlanDataSourceKnownPlan(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}
		plan := rs.Primary.Attributes["plan"]
		if !strings.EqualFold(plan, registry.CrPlanFree) && !strings.EqualFold(plan, registry.CrPlanStandard) {
			return fmt.Errorf("%s: expected plan to be %q or %q, got %q", n, registry.CrPlanFree, registry.CrPlanStandard, plan)
		}
		return nil
	}
}
