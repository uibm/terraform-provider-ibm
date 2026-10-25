// Copyright IBM Corp. 2026 All Rights Reserved.
// Licensed under the Mozilla Public License v2.0

package secretsmanager_test

import (
	"fmt"
	"testing"

	acc "github.com/IBM-Cloud/terraform-provider-ibm/ibm/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

// Creates an arbitrary secret, then rotates it with the action and checks the
// secret has a second version.
func TestAccIBMSmArbitrarySecretRotateAction(t *testing.T) {
	name := fmt.Sprintf("tf-rotate-%d", acctest.RandIntRange(10, 100000))
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acc.TestAccPreCheck(t) },
		ProtoV6ProviderFactories: acc.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
					resource "ibm_sm_arbitrary_secret" "test" {
						instance_id = "%[1]s"
						region      = "%[2]s"
						name        = "%[3]s"
						payload     = "first-value"
						lifecycle {
							ignore_changes = [payload]
						}
					}

					action "ibm_sm_arbitrary_secret_rotate" "test" {
						config {
							instance_id = "%[1]s"
							region      = "%[2]s"
							secret_id   = ibm_sm_arbitrary_secret.test.secret_id
							payload     = "second-value"
						}
					}

					resource "terraform_data" "rotate" {
						input = ibm_sm_arbitrary_secret.test.secret_id
						lifecycle {
							action_trigger {
								events  = [after_create]
								actions = [action.ibm_sm_arbitrary_secret_rotate.test]
							}
						}
					}
				`, acc.SecretsManagerInstanceID, acc.SecretsManagerInstanceRegion, name),
			},
			{
				RefreshState: true,
				Check:        resource.TestCheckResourceAttr("ibm_sm_arbitrary_secret.test", "versions_total", "2"),
			},
		},
	})
}
