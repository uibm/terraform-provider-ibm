// Copyright IBM Corp. 2026 All Rights Reserved.
// Licensed under the Mozilla Public License v2.0

package registry

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"

	"github.com/IBM-Cloud/terraform-provider-ibm/ibm/conns"
	"github.com/IBM-Cloud/terraform-provider-ibm/ibm/flex"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"

	"github.com/IBM/container-registry-go-sdk/containerregistryv1"
)

const (
	// CrPlanFree is the default (no charge) Container Registry pricing plan.
	CrPlanFree = "Free"
	// CrPlanStandard is the paid Container Registry pricing plan with unlimited storage and pull traffic.
	CrPlanStandard = "Standard"
)

// The plan and the quotas are account-wide settings that are scoped to a single
// registry instance (the regional or global registry the provider targets). There is
// no create or delete API for them: they always exist and can only be read or updated.
func ResourceIBMCrPlan() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceIBMCrPlanCreate,
		ReadContext:   resourceIBMCrPlanRead,
		UpdateContext: resourceIBMCrPlanUpdate,
		DeleteContext: resourceIBMCrPlanDelete,
		Importer:      &schema.ResourceImporter{},

		Schema: map[string]*schema.Schema{
			"plan": {
				Type:             schema.TypeString,
				Required:         true,
				ValidateFunc:     validation.StringInSlice([]string{CrPlanStandard, CrPlanFree}, true),
				DiffSuppressFunc: suppressCrPlanCaseDiff,
				Description:      "The pricing plan of the targeted registry for the IBM Cloud account. Allowed values are 'Standard' and 'Free'. The value is compared case-insensitively.",
			},
			"registry": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The host name of the registry instance to which the plan applies, for example 'us.icr.io'.",
			},
		},
	}
}

func resourceIBMCrPlanCreate(context context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	containerRegistryClient, err := meta.(conns.ClientSession).ContainerRegistryV1()
	if err != nil {
		tfErr := flex.TerraformErrorf(err, err.Error(), "ibm_cr_plan", "create")
		log.Printf("[DEBUG]\n%s", tfErr.GetDebugMessage())
		return tfErr.GetDiag()
	}

	if diags := crPlanApply(context, containerRegistryClient, d.Get("plan").(string), "create"); diags != nil {
		return diags
	}

	d.SetId(crRegistryHost(containerRegistryClient))

	return resourceIBMCrPlanRead(context, d, meta)
}

func resourceIBMCrPlanRead(context context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	containerRegistryClient, err := meta.(conns.ClientSession).ContainerRegistryV1()
	if err != nil {
		tfErr := flex.TerraformErrorf(err, err.Error(), "ibm_cr_plan", "read")
		log.Printf("[DEBUG]\n%s", tfErr.GetDebugMessage())
		return tfErr.GetDiag()
	}

	plan, response, err := containerRegistryClient.GetPlansWithContext(context, &containerregistryv1.GetPlansOptions{})
	if err != nil {
		tfErr := flex.TerraformErrorf(err, fmt.Sprintf("GetPlansWithContext failed: %s\n%s", err.Error(), response), "ibm_cr_plan", "read")
		log.Printf("[DEBUG]\n%s", tfErr.GetDebugMessage())
		return tfErr.GetDiag()
	}
	if plan == nil || plan.Plan == nil {
		tfErr := flex.TerraformErrorf(fmt.Errorf("the registry returned an empty plan"), "GetPlansWithContext returned no plan", "ibm_cr_plan", "read")
		log.Printf("[DEBUG]\n%s", tfErr.GetDebugMessage())
		return tfErr.GetDiag()
	}

	// The registry is a singleton per provider configuration, so the identifier is always
	// re-derived here. This keeps imports with an arbitrary identifier consistent.
	d.SetId(crRegistryHost(containerRegistryClient))

	if err = d.Set("plan", *plan.Plan); err != nil {
		return flex.DiscriminatedTerraformErrorf(err, fmt.Sprintf("Error setting plan: %s", err), "ibm_cr_plan", "read", "set-plan").GetDiag()
	}
	if err = d.Set("registry", d.Id()); err != nil {
		return flex.DiscriminatedTerraformErrorf(err, fmt.Sprintf("Error setting registry: %s", err), "ibm_cr_plan", "read", "set-registry").GetDiag()
	}

	return nil
}

func resourceIBMCrPlanUpdate(context context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	containerRegistryClient, err := meta.(conns.ClientSession).ContainerRegistryV1()
	if err != nil {
		tfErr := flex.TerraformErrorf(err, err.Error(), "ibm_cr_plan", "update")
		log.Printf("[DEBUG]\n%s", tfErr.GetDebugMessage())
		return tfErr.GetDiag()
	}

	if d.HasChange("plan") {
		if diags := crPlanApply(context, containerRegistryClient, d.Get("plan").(string), "update"); diags != nil {
			return diags
		}
	}

	return resourceIBMCrPlanRead(context, d, meta)
}

// The plan cannot be removed from an account; deleting the resource only stops
// Terraform from managing it and leaves the current plan in place.
func resourceIBMCrPlanDelete(context context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	d.SetId("")

	return nil
}

// crPlanApply updates the plan of the targeted registry unless the registry already
// reports the requested plan, so that re-applying the same plan never issues a change.
func crPlanApply(context context.Context, client *containerregistryv1.ContainerRegistryV1, requestedPlan string, operation string) diag.Diagnostics {
	current, response, err := client.GetPlansWithContext(context, &containerregistryv1.GetPlansOptions{})
	if err != nil {
		tfErr := flex.TerraformErrorf(err, fmt.Sprintf("GetPlansWithContext failed: %s\n%s", err.Error(), response), "ibm_cr_plan", operation)
		log.Printf("[DEBUG]\n%s", tfErr.GetDebugMessage())
		return tfErr.GetDiag()
	}
	if current != nil && current.Plan != nil && strings.EqualFold(*current.Plan, requestedPlan) {
		log.Printf("[DEBUG] registry plan is already %q, no update required", *current.Plan)
		return nil
	}

	updatePlansOptions := &containerregistryv1.UpdatePlansOptions{}
	updatePlansOptions.SetPlan(requestedPlan)

	response, err = client.UpdatePlansWithContext(context, updatePlansOptions)
	if err != nil {
		tfErr := flex.TerraformErrorf(err, fmt.Sprintf("UpdatePlansWithContext failed: %s\n%s", err.Error(), response), "ibm_cr_plan", operation)
		log.Printf("[DEBUG]\n%s", tfErr.GetDebugMessage())
		return tfErr.GetDiag()
	}

	return nil
}

// The registry reports plan names capitalized ("Standard") while users commonly write
// them in lower case ("standard"), as the CLI does. Both spellings denote the same plan.
func suppressCrPlanCaseDiff(k, oldValue, newValue string, d *schema.ResourceData) bool {
	return strings.EqualFold(oldValue, newValue)
}

// crRegistryHost returns the host name of the registry the client targets, for example
// "us.icr.io". It is used as the identifier of the account-wide registry settings.
func crRegistryHost(client *containerregistryv1.ContainerRegistryV1) string {
	serviceURL := client.GetServiceURL()
	if parsed, err := url.Parse(serviceURL); err == nil && parsed.Host != "" {
		return parsed.Host
	}
	return strings.TrimPrefix(strings.TrimPrefix(serviceURL, "https://"), "http://")
}
