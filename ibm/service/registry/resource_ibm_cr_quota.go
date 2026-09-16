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
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"

	"github.com/IBM/container-registry-go-sdk/containerregistryv1"
)

const (
	// CrQuotaUnlimited is the quota value that denotes "Unlimited" for both the megabyte
	// values accepted by the update API and the byte values returned by the get API.
	CrQuotaUnlimited int64 = -1

	// The registry converts megabyte quota settings to bytes with a binary megabyte
	// (500 MB is reported as 524288000 bytes).
	crQuotaBytesPerMegabyte int64 = 1024 * 1024
)

// The quotas are account-wide settings that are scoped to a single registry instance
// (the regional or global registry the provider targets). They always exist and can
// only be read or updated; there is no API to create or delete them.
func ResourceIBMCrQuota() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceIBMCrQuotaCreate,
		ReadContext:   resourceIBMCrQuotaRead,
		UpdateContext: resourceIBMCrQuotaUpdate,
		DeleteContext: resourceIBMCrQuotaDelete,
		Importer:      &schema.ResourceImporter{},

		Schema: map[string]*schema.Schema{
			"storage_megabytes": {
				Type:         schema.TypeInt,
				Optional:     true,
				Computed:     true,
				ValidateFunc: validation.IntAtLeast(int(CrQuotaUnlimited)),
				AtLeastOneOf: []string{"storage_megabytes", "traffic_megabytes"},
				Description:  "The storage quota in megabytes. The value -1 denotes 'Unlimited'. When omitted the current storage quota of the registry is kept.",
			},
			"traffic_megabytes": {
				Type:         schema.TypeInt,
				Optional:     true,
				Computed:     true,
				ValidateFunc: validation.IntAtLeast(int(CrQuotaUnlimited)),
				AtLeastOneOf: []string{"storage_megabytes", "traffic_megabytes"},
				Description:  "The pull traffic quota in megabytes per month. The value -1 denotes 'Unlimited'. When omitted the current traffic quota of the registry is kept.",
			},
			"registry": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The host name of the registry instance to which the quotas apply, for example 'us.icr.io'.",
			},
		},
	}
}

func resourceIBMCrQuotaCreate(context context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	containerRegistryClient, err := meta.(conns.ClientSession).ContainerRegistryV1()
	if err != nil {
		tfErr := flex.TerraformErrorf(err, err.Error(), "ibm_cr_quota", "create")
		log.Printf("[DEBUG]\n%s", tfErr.GetDebugMessage())
		return tfErr.GetDiag()
	}

	updateQuotaOptions := &containerregistryv1.UpdateQuotaOptions{}

	// GetOk cannot be used here because 0 is a meaningful quota value, so the raw
	// configuration is consulted to find out which arguments the user actually set.
	rawConfig := d.GetRawConfig()
	if !rawConfig.IsNull() && !rawConfig.GetAttr("storage_megabytes").IsNull() {
		updateQuotaOptions.SetStorageMegabytes(int64(d.Get("storage_megabytes").(int)))
	}
	if !rawConfig.IsNull() && !rawConfig.GetAttr("traffic_megabytes").IsNull() {
		updateQuotaOptions.SetTrafficMegabytes(int64(d.Get("traffic_megabytes").(int)))
	}

	if updateQuotaOptions.StorageMegabytes != nil || updateQuotaOptions.TrafficMegabytes != nil {
		response, err := containerRegistryClient.UpdateQuotaWithContext(context, updateQuotaOptions)
		if err != nil {
			tfErr := flex.TerraformErrorf(err, fmt.Sprintf("UpdateQuotaWithContext failed: %s\n%s", err.Error(), response), "ibm_cr_quota", "create")
			log.Printf("[DEBUG]\n%s", tfErr.GetDebugMessage())
			return tfErr.GetDiag()
		}
	}

	d.SetId(crRegistryHost(containerRegistryClient))

	return resourceIBMCrQuotaRead(context, d, meta)
}

func resourceIBMCrQuotaRead(context context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	containerRegistryClient, err := meta.(conns.ClientSession).ContainerRegistryV1()
	if err != nil {
		tfErr := flex.TerraformErrorf(err, err.Error(), "ibm_cr_quota", "read")
		log.Printf("[DEBUG]\n%s", tfErr.GetDebugMessage())
		return tfErr.GetDiag()
	}

	quota, response, err := containerRegistryClient.GetQuotaWithContext(context, &containerregistryv1.GetQuotaOptions{})
	if err != nil {
		tfErr := flex.TerraformErrorf(err, fmt.Sprintf("GetQuotaWithContext failed: %s\n%s", err.Error(), response), "ibm_cr_quota", "read")
		log.Printf("[DEBUG]\n%s", tfErr.GetDebugMessage())
		return tfErr.GetDiag()
	}
	if quota == nil || quota.Limit == nil {
		tfErr := flex.TerraformErrorf(fmt.Errorf("the registry returned no quota limits"), "GetQuotaWithContext returned no quota limits", "ibm_cr_quota", "read")
		log.Printf("[DEBUG]\n%s", tfErr.GetDebugMessage())
		return tfErr.GetDiag()
	}

	// The quotas are a singleton per provider configuration, so the identifier is always
	// re-derived here. This keeps imports with an arbitrary identifier consistent.
	d.SetId(crRegistryHost(containerRegistryClient))

	if quota.Limit.StorageBytes != nil {
		if err = d.Set("storage_megabytes", crQuotaBytesToMegabytes(*quota.Limit.StorageBytes)); err != nil {
			return flex.DiscriminatedTerraformErrorf(err, fmt.Sprintf("Error setting storage_megabytes: %s", err), "ibm_cr_quota", "read", "set-storage_megabytes").GetDiag()
		}
	}
	if quota.Limit.TrafficBytes != nil {
		if err = d.Set("traffic_megabytes", crQuotaBytesToMegabytes(*quota.Limit.TrafficBytes)); err != nil {
			return flex.DiscriminatedTerraformErrorf(err, fmt.Sprintf("Error setting traffic_megabytes: %s", err), "ibm_cr_quota", "read", "set-traffic_megabytes").GetDiag()
		}
	}
	if err = d.Set("registry", d.Id()); err != nil {
		return flex.DiscriminatedTerraformErrorf(err, fmt.Sprintf("Error setting registry: %s", err), "ibm_cr_quota", "read", "set-registry").GetDiag()
	}

	return nil
}

func resourceIBMCrQuotaUpdate(context context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	containerRegistryClient, err := meta.(conns.ClientSession).ContainerRegistryV1()
	if err != nil {
		tfErr := flex.TerraformErrorf(err, err.Error(), "ibm_cr_quota", "update")
		log.Printf("[DEBUG]\n%s", tfErr.GetDebugMessage())
		return tfErr.GetDiag()
	}

	updateQuotaOptions := &containerregistryv1.UpdateQuotaOptions{}

	if d.HasChange("storage_megabytes") {
		updateQuotaOptions.SetStorageMegabytes(int64(d.Get("storage_megabytes").(int)))
	}
	if d.HasChange("traffic_megabytes") {
		updateQuotaOptions.SetTrafficMegabytes(int64(d.Get("traffic_megabytes").(int)))
	}

	if updateQuotaOptions.StorageMegabytes != nil || updateQuotaOptions.TrafficMegabytes != nil {
		response, err := containerRegistryClient.UpdateQuotaWithContext(context, updateQuotaOptions)
		if err != nil {
			tfErr := flex.TerraformErrorf(err, fmt.Sprintf("UpdateQuotaWithContext failed: %s\n%s", err.Error(), response), "ibm_cr_quota", "update")
			log.Printf("[DEBUG]\n%s", tfErr.GetDebugMessage())
			return tfErr.GetDiag()
		}
	}

	return resourceIBMCrQuotaRead(context, d, meta)
}

// Quotas cannot be removed from an account and their defaults depend on the plan, so
// deleting the resource only stops Terraform from managing them and leaves the current
// limits in place.
func resourceIBMCrQuotaDelete(context context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	d.SetId("")

	return nil
}

// crQuotaBytesToMegabytes converts a quota limit reported in bytes to the megabyte value
// accepted by the update API, preserving the "Unlimited" marker.
func crQuotaBytesToMegabytes(bytes int64) int {
	if bytes < 0 {
		return int(CrQuotaUnlimited)
	}
	return int(bytes / crQuotaBytesPerMegabyte)
}
