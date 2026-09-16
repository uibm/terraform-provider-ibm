// Copyright IBM Corp. 2026 All Rights Reserved.
// Licensed under the Mozilla Public License v2.0

package registry

import (
	"context"
	"fmt"
	"log"

	"github.com/IBM-Cloud/terraform-provider-ibm/ibm/conns"
	"github.com/IBM-Cloud/terraform-provider-ibm/ibm/flex"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/IBM/container-registry-go-sdk/containerregistryv1"
)

func DataSourceIBMCrPlan() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceIBMCrPlanRead,

		Schema: map[string]*schema.Schema{
			"plan": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The pricing plan of the targeted registry for the IBM Cloud account, for example 'Standard' or 'Free'.",
			},
			"registry": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The host name of the registry instance to which the plan applies, for example 'us.icr.io'.",
			},
		},
	}
}

func dataSourceIBMCrPlanRead(context context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	containerRegistryClient, err := meta.(conns.ClientSession).ContainerRegistryV1()
	if err != nil {
		tfErr := flex.TerraformErrorf(err, err.Error(), "(Data) ibm_cr_plan", "read")
		log.Printf("[DEBUG]\n%s", tfErr.GetDebugMessage())
		return tfErr.GetDiag()
	}

	plan, response, err := containerRegistryClient.GetPlansWithContext(context, &containerregistryv1.GetPlansOptions{})
	if err != nil {
		tfErr := flex.TerraformErrorf(err, fmt.Sprintf("GetPlansWithContext failed: %s\n%s", err.Error(), response), "(Data) ibm_cr_plan", "read")
		log.Printf("[DEBUG]\n%s", tfErr.GetDebugMessage())
		return tfErr.GetDiag()
	}
	if plan == nil || plan.Plan == nil {
		tfErr := flex.TerraformErrorf(fmt.Errorf("the registry returned an empty plan"), "GetPlansWithContext returned no plan", "(Data) ibm_cr_plan", "read")
		log.Printf("[DEBUG]\n%s", tfErr.GetDebugMessage())
		return tfErr.GetDiag()
	}

	d.SetId(crRegistryHost(containerRegistryClient))

	if err = d.Set("plan", *plan.Plan); err != nil {
		return flex.DiscriminatedTerraformErrorf(err, fmt.Sprintf("Error setting plan: %s", err), "(Data) ibm_cr_plan", "read", "set-plan").GetDiag()
	}
	if err = d.Set("registry", d.Id()); err != nil {
		return flex.DiscriminatedTerraformErrorf(err, fmt.Sprintf("Error setting registry: %s", err), "(Data) ibm_cr_plan", "read", "set-registry").GetDiag()
	}

	return nil
}
