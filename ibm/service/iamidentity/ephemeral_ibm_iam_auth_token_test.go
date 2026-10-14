// Copyright IBM Corp. 2026 All Rights Reserved.
// Licensed under the Mozilla Public License v2.0

package iamidentity_test

import (
	"fmt"
	"testing"

	acc "github.com/IBM-Cloud/terraform-provider-ibm/ibm/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// TestAccIBMIAMAuthTokenEphemeralBasic opens the ephemeral token during plan and
// apply and checks that nothing about it is written to state.
func TestAccIBMIAMAuthTokenEphemeralBasic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acc.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acc.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: `ephemeral "ibm_iam_auth_token" "token" {}`,
				Check:  testAccCheckNoTokenInState,
			},
		},
	})
}

func testAccCheckNoTokenInState(s *terraform.State) error {
	for name := range s.RootModule().Resources {
		return fmt.Errorf("expected empty state for ephemeral resource, found %s", name)
	}
	return nil
}
