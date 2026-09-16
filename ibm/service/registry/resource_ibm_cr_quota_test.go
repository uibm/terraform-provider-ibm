// Copyright IBM Corp. 2026 All Rights Reserved.
// Licensed under the Mozilla Public License v2.0

package registry_test

import (
	"fmt"
	"strconv"
	"testing"

	acc "github.com/IBM-Cloud/terraform-provider-ibm/ibm/acctest"
	"github.com/IBM-Cloud/terraform-provider-ibm/ibm/conns"
	"github.com/IBM-Cloud/terraform-provider-ibm/ibm/service/registry"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"

	"github.com/IBM/container-registry-go-sdk/containerregistryv1"
)

// Changing the quotas is an account-wide operation that stays in effect after the test,
// so the test only runs when IBM_CR_QUOTA_STORAGE_MEGABYTES and
// IBM_CR_QUOTA_TRAFFIC_MEGABYTES name the limits the targeted registry should end up with.
func TestAccIBMCrQuotaBasic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acc.TestAccPreCheck(t)
			if acc.CrQuotaStorageMegabytes == "" || acc.CrQuotaTrafficMegabytes == "" {
				t.Skip("IBM_CR_QUOTA_STORAGE_MEGABYTES or IBM_CR_QUOTA_TRAFFIC_MEGABYTES is not set; skipping ibm_cr_quota resource test")
			}
		},
		Providers:    acc.TestAccProviders,
		CheckDestroy: testAccCheckIBMCrQuotaDestroy,
		Steps: []resource.TestStep{
			{
				// Only the storage quota is configured; the traffic quota must be read
				// from the registry instead of being reset.
				Config: testAccCheckIBMCrQuotaConfigStorageOnly(acc.CrQuotaStorageMegabytes),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckIBMCrQuotaExists("ibm_cr_quota.cr_quota"),
					resource.TestCheckResourceAttr("ibm_cr_quota.cr_quota", "storage_megabytes", acc.CrQuotaStorageMegabytes),
					resource.TestCheckResourceAttrSet("ibm_cr_quota.cr_quota", "traffic_megabytes"),
					resource.TestCheckResourceAttrSet("ibm_cr_quota.cr_quota", "registry"),
					resource.TestCheckResourceAttrPair("ibm_cr_quota.cr_quota", "id", "ibm_cr_quota.cr_quota", "registry"),
				),
			},
			{
				Config: testAccCheckIBMCrQuotaConfig(acc.CrQuotaStorageMegabytes, acc.CrQuotaTrafficMegabytes),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckIBMCrQuotaExists("ibm_cr_quota.cr_quota"),
					resource.TestCheckResourceAttr("ibm_cr_quota.cr_quota", "storage_megabytes", acc.CrQuotaStorageMegabytes),
					resource.TestCheckResourceAttr("ibm_cr_quota.cr_quota", "traffic_megabytes", acc.CrQuotaTrafficMegabytes),
				),
			},
			{
				ResourceName:      "ibm_cr_quota.cr_quota",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCheckIBMCrQuotaConfigStorageOnly(storageMegabytes string) string {
	return fmt.Sprintf(`
		resource "ibm_cr_quota" "cr_quota" {
			storage_megabytes = %s
		}
	`, storageMegabytes)
}

func testAccCheckIBMCrQuotaConfig(storageMegabytes string, trafficMegabytes string) string {
	return fmt.Sprintf(`
		resource "ibm_cr_quota" "cr_quota" {
			storage_megabytes = %s
			traffic_megabytes = %s
		}
	`, storageMegabytes, trafficMegabytes)
}

// testAccCheckIBMCrQuotaExists verifies that the limits stored in state match the limits
// the registry reports, after converting the reported bytes to megabytes.
func testAccCheckIBMCrQuotaExists(n string) resource.TestCheckFunc {
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

		quota, _, err := containerRegistryClient.GetQuota(&containerregistryv1.GetQuotaOptions{})
		if err != nil {
			return err
		}
		if quota == nil || quota.Limit == nil {
			return fmt.Errorf("the registry returned no quota limits")
		}

		if err := testAccCompareIBMCrQuotaLimit(rs.Primary.Attributes["storage_megabytes"], quota.Limit.StorageBytes, "storage"); err != nil {
			return err
		}
		return testAccCompareIBMCrQuotaLimit(rs.Primary.Attributes["traffic_megabytes"], quota.Limit.TrafficBytes, "traffic")
	}
}

func testAccCompareIBMCrQuotaLimit(stateMegabytes string, limitBytes *int64, name string) error {
	if limitBytes == nil {
		return fmt.Errorf("the registry returned no %s quota limit", name)
	}
	stateValue, err := strconv.ParseInt(stateMegabytes, 10, 64)
	if err != nil {
		return fmt.Errorf("state %s_megabytes %q is not an integer: %s", name, stateMegabytes, err)
	}
	expected := int64(registry.CrQuotaUnlimited)
	if *limitBytes >= 0 {
		expected = *limitBytes / (1024 * 1024)
	}
	if stateValue != expected {
		return fmt.Errorf("expected %s_megabytes %d (registry reports %d bytes), got %d", name, expected, *limitBytes, stateValue)
	}
	return nil
}

// Quotas cannot be removed from an account, so destroying the resource only removes it
// from state and the registry keeps reporting its limits.
func testAccCheckIBMCrQuotaDestroy(s *terraform.State) error {
	containerRegistryClient, err := acc.TestAccProvider.Meta().(conns.ClientSession).ContainerRegistryV1()
	if err != nil {
		return err
	}
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "ibm_cr_quota" {
			continue
		}

		quota, _, err := containerRegistryClient.GetQuota(&containerregistryv1.GetQuotaOptions{})
		if err != nil {
			return fmt.Errorf("[ERROR] Error reading the registry quotas after destroying cr_quota (%s): %s", rs.Primary.ID, err)
		}
		if quota == nil || quota.Limit == nil {
			return fmt.Errorf("the registry quota limits disappeared after destroying cr_quota (%s)", rs.Primary.ID)
		}
	}

	return nil
}
