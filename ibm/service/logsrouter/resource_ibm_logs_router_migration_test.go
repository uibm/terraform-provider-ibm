// Copyright IBM Corp. 2026 All Rights Reserved.
// Licensed under the Mozilla Public License v2.0

package logsrouter_test

import (
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"

	acc "github.com/IBM-Cloud/terraform-provider-ibm/ibm/acctest"
)

// The migration changes the whole account and completion cannot be undone, so
// these tests run only against an account prepared for them:
//   - IBM_LOGS_ROUTER_MIGRATION_ACC=1: account in state BEFORE with primary_metadata_region
//     set. Runs generate, then reset (does not complete).
//   - IBM_LOGS_ROUTER_MIGRATION_COMPLETE_ACC=1: additionally completes the migration.
func migrationPreCheck(t *testing.T, env string) {
	if os.Getenv(env) == "" {
		t.Skipf("set %s=1 to run; this test changes the account's Logs Routing migration", env)
	}
	acc.TestAccPreCheck(t)
}

const migrationRes = "ibm_logs_router_migration.migration"

func TestAccIBMLogsRouterMigrationGenerateReset(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:  func() { migrationPreCheck(t, "IBM_LOGS_ROUTER_MIGRATION_ACC") },
		Providers: acc.TestAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccIBMLogsRouterMigrationConfig(false, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(migrationRes, "state", "PENDING_COMPLETION"),
					resource.TestCheckResourceAttr(migrationRes, "complete", "false"),
					resource.TestCheckResourceAttrPair("data.ibm_logs_router_migration.status", "state", migrationRes, "state"),
					resource.TestCheckResourceAttrSet("data.ibm_logs_router_targets.generated", "targets.#"),
				),
			},
			{
				// A second plan has no diff.
				Config:   testAccIBMLogsRouterMigrationConfig(false, true),
				PlanOnly: true,
			},
			{
				ResourceName:      migrationRes,
				ImportState:       true,
				ImportStateId:     "logs-router-migration",
				ImportStateVerify: true,
				// reset_on_destroy is a Terraform-only setting.
				ImportStateVerifyIgnore: []string{"reset_on_destroy"},
			},
		},
		// reset_on_destroy = true: destroy resets the migration to BEFORE.
	})
}

func TestAccIBMLogsRouterMigrationComplete(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:  func() { migrationPreCheck(t, "IBM_LOGS_ROUTER_MIGRATION_COMPLETE_ACC") },
		Providers: acc.TestAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccIBMLogsRouterMigrationConfig(false, false),
				Check:  resource.TestCheckResourceAttr(migrationRes, "state", "PENDING_COMPLETION"),
			},
			{
				Config: testAccIBMLogsRouterMigrationConfig(true, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(migrationRes, "state", "COMPLETE"),
					resource.TestCheckResourceAttr(migrationRes, "api_version", "3"),
				),
			},
			{
				Config:      testAccIBMLogsRouterMigrationConfig(false, false),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile("cannot be changed from true to false"),
			},
			{
				ResourceName:      migrationRes,
				ImportState:       true,
				ImportStateId:     "logs-router-migration",
				ImportStateVerify: true,
			},
		},
		// Destroy in COMPLETE only removes the resource from state, with a warning.
	})
}

func TestAccIBMLogsRouterMigrationDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:  func() { acc.TestAccPreCheck(t) },
		Providers: acc.TestAccProviders,
		Steps: []resource.TestStep{
			{
				Config: `data "ibm_logs_router_migration" "status" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestMatchResourceAttr("data.ibm_logs_router_migration.status", "state", regexp.MustCompile("^(BEFORE|IN_PROGRESS|PENDING_COMPLETION|COMPLETE)$")),
					resource.TestCheckResourceAttrSet("data.ibm_logs_router_migration.status", "api_version"),
				),
			},
		},
	})
}

func TestAccIBMLogsRouterDestinationsDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:  func() { acc.TestAccPreCheck(t) },
		Providers: acc.TestAccProviders,
		Steps: []resource.TestStep{
			{
				Config: `data "ibm_logs_router_destinations" "query" {
  crns = ["` + iclDestinationCRN + `"]
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.ibm_logs_router_destinations.query", "destinations.#", "1"),
					resource.TestCheckResourceAttr("data.ibm_logs_router_destinations.query", "destinations.0.crn", iclDestinationCRN),
				),
			},
		},
	})
}

func testAccIBMLogsRouterMigrationConfig(complete, resetOnDestroy bool) string {
	c, r := "false", "false"
	if complete {
		c = "true"
	}
	if resetOnDestroy {
		r = "true"
	}
	return `
resource "ibm_logs_router_migration" "migration" {
  complete         = ` + c + `
  reset_on_destroy = ` + r + `
}

data "ibm_logs_router_migration" "status" {
  depends_on = [ibm_logs_router_migration.migration]
}

data "ibm_logs_router_targets" "generated" {
  depends_on = [ibm_logs_router_migration.migration]
}
`
}
