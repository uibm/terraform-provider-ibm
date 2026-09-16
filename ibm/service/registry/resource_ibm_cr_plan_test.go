// Copyright IBM Corp. 2026 All Rights Reserved.
// Licensed under the Mozilla Public License v2.0

package registry_test

import (
	"fmt"
	"strings"
	"testing"

	acc "github.com/IBM-Cloud/terraform-provider-ibm/ibm/acctest"
	"github.com/IBM-Cloud/terraform-provider-ibm/ibm/conns"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"

	"github.com/IBM/container-registry-go-sdk/containerregistryv1"
)

// Changing the plan is a billing-relevant, account-wide operation, so the test only runs
// when IBM_CR_PLAN names the plan the targeted registry should have (for example "Standard").
func TestAccIBMCrPlanBasic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acc.TestAccPreCheck(t)
			if acc.CrPlan == "" {
				t.Skip("IBM_CR_PLAN is not set; skipping ibm_cr_plan resource test")
			}
		},
		Providers:    acc.TestAccProviders,
		CheckDestroy: testAccCheckIBMCrPlanDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCheckIBMCrPlanConfig(acc.CrPlan),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckIBMCrPlanExists("ibm_cr_plan.cr_plan", acc.CrPlan),
					testAccCheckIBMCrPlanStateAttr("ibm_cr_plan.cr_plan", acc.CrPlan),
					resource.TestCheckResourceAttrSet("ibm_cr_plan.cr_plan", "registry"),
					resource.TestCheckResourceAttrPair("ibm_cr_plan.cr_plan", "id", "ibm_cr_plan.cr_plan", "registry"),
				),
			},
			{
				// The plan name is compared case-insensitively, so a lower-case spelling
				// of the same plan must not produce a diff.
				Config:   testAccCheckIBMCrPlanConfig(strings.ToLower(acc.CrPlan)),
				PlanOnly: true,
			},
			{
				ResourceName:      "ibm_cr_plan.cr_plan",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCheckIBMCrPlanConfig(plan string) string {
	return fmt.Sprintf(`
		resource "ibm_cr_plan" "cr_plan" {
			plan = "%s"
		}
	`, plan)
}

// The registry reports plan names capitalized regardless of the spelling that was sent,
// so the state value is compared case-insensitively with the configured plan.
func testAccCheckIBMCrPlanStateAttr(n string, expectedPlan string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}
		if got := rs.Primary.Attributes["plan"]; !strings.EqualFold(got, expectedPlan) {
			return fmt.Errorf("%s: attribute plan expected %q (case-insensitive), got %q", n, expectedPlan, got)
		}
		return nil
	}
}

func testAccCheckIBMCrPlanExists(n string, expectedPlan string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("%s has no ID set", n)
		}

		containerRegistryClient, err := acc.TestAccProvider.Meta().(conns.ClientSession).ContainerRegistryV1()
		if err != nil {
			return err
		}

		plan, _, err := containerRegistryClient.GetPlans(&containerregistryv1.GetPlansOptions{})
		if err != nil {
			return err
		}
		if plan == nil || plan.Plan == nil {
			return fmt.Errorf("the registry returned no plan")
		}
		if !strings.EqualFold(*plan.Plan, expectedPlan) {
			return fmt.Errorf("expected registry plan %q, got %q", expectedPlan, *plan.Plan)
		}
		return nil
	}
}

// The plan cannot be removed from an account, so destroying the resource only removes it
// from state and the registry keeps reporting a plan.
func testAccCheckIBMCrPlanDestroy(s *terraform.State) error {
	containerRegistryClient, err := acc.TestAccProvider.Meta().(conns.ClientSession).ContainerRegistryV1()
	if err != nil {
		return err
	}
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "ibm_cr_plan" {
			continue
		}

		plan, _, err := containerRegistryClient.GetPlans(&containerregistryv1.GetPlansOptions{})
		if err != nil {
			return fmt.Errorf("[ERROR] Error reading the registry plan after destroying cr_plan (%s): %s", rs.Primary.ID, err)
		}
		if plan == nil || plan.Plan == nil || *plan.Plan == "" {
			return fmt.Errorf("the registry plan disappeared after destroying cr_plan (%s)", rs.Primary.ID)
		}
	}

	return nil
}
