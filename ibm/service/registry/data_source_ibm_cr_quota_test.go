// Copyright IBM Corp. 2026 All Rights Reserved.
// Licensed under the Mozilla Public License v2.0

package registry_test

import (
	"fmt"
	"strconv"
	"testing"

	acc "github.com/IBM-Cloud/terraform-provider-ibm/ibm/acctest"
	"github.com/IBM-Cloud/terraform-provider-ibm/ibm/service/registry"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccIBMCrQuotaDataSourceBasic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:  func() { acc.TestAccPreCheck(t) },
		Providers: acc.TestAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccCheckIBMCrQuotaDataSourceConfigBasic(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.ibm_cr_quota.cr_quota", "id"),
					resource.TestCheckResourceAttrSet("data.ibm_cr_quota.cr_quota", "registry"),
					resource.TestCheckResourceAttrPair("data.ibm_cr_quota.cr_quota", "id", "data.ibm_cr_quota.cr_quota", "registry"),
					resource.TestCheckResourceAttrSet("data.ibm_cr_quota.cr_quota", "storage_megabytes"),
					resource.TestCheckResourceAttrSet("data.ibm_cr_quota.cr_quota", "traffic_megabytes"),
					resource.TestCheckResourceAttr("data.ibm_cr_quota.cr_quota", "limit.#", "1"),
					resource.TestCheckResourceAttrSet("data.ibm_cr_quota.cr_quota", "limit.0.storage_bytes"),
					resource.TestCheckResourceAttrSet("data.ibm_cr_quota.cr_quota", "limit.0.traffic_bytes"),
					resource.TestCheckResourceAttr("data.ibm_cr_quota.cr_quota", "usage.#", "1"),
					testAccCheckIBMCrQuotaDataSourceMegabytesMatchBytes("data.ibm_cr_quota.cr_quota", "storage"),
					testAccCheckIBMCrQuotaDataSourceMegabytesMatchBytes("data.ibm_cr_quota.cr_quota", "traffic"),
				),
			},
		},
	})
}

func testAccCheckIBMCrQuotaDataSourceConfigBasic() string {
	return `
		data "ibm_cr_quota" "cr_quota" {
		}
	`
}

// The megabyte convenience attributes must be derived from the byte limits, with -1
// preserved as the "Unlimited" marker.
func testAccCheckIBMCrQuotaDataSourceMegabytesMatchBytes(n string, name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}
		megabytes, err := strconv.ParseInt(rs.Primary.Attributes[name+"_megabytes"], 10, 64)
		if err != nil {
			return fmt.Errorf("%s: %s_megabytes is not an integer: %s", n, name, err)
		}
		bytes, err := strconv.ParseInt(rs.Primary.Attributes["limit.0."+name+"_bytes"], 10, 64)
		if err != nil {
			return fmt.Errorf("%s: limit.0.%s_bytes is not an integer: %s", n, name, err)
		}
		expected := int64(registry.CrQuotaUnlimited)
		if bytes >= 0 {
			expected = bytes / (1024 * 1024)
		}
		if megabytes != expected {
			return fmt.Errorf("%s: expected %s_megabytes %d for %d bytes, got %d", n, name, expected, bytes, megabytes)
		}
		return nil
	}
}
