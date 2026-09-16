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

func DataSourceIBMCrQuota() *schema.Resource {
	quotaDetailsSchema := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"storage_bytes": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Storage quota or usage in bytes. The value -1 denotes 'Unlimited'.",
			},
			"traffic_bytes": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Pull traffic quota or usage in bytes. The value -1 denotes 'Unlimited'.",
			},
		},
	}

	return &schema.Resource{
		ReadContext: dataSourceIBMCrQuotaRead,

		Schema: map[string]*schema.Schema{
			"storage_megabytes": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The storage quota in megabytes. The value -1 denotes 'Unlimited'.",
			},
			"traffic_megabytes": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The pull traffic quota in megabytes per month. The value -1 denotes 'Unlimited'.",
			},
			"limit": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "The quota limits of the targeted registry for the IBM Cloud account.",
				Elem:        quotaDetailsSchema,
			},
			"usage": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "The current usage of the targeted registry for the IBM Cloud account.",
				Elem:        quotaDetailsSchema,
			},
			"registry": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The host name of the registry instance to which the quotas apply, for example 'us.icr.io'.",
			},
		},
	}
}

func dataSourceIBMCrQuotaRead(context context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	containerRegistryClient, err := meta.(conns.ClientSession).ContainerRegistryV1()
	if err != nil {
		tfErr := flex.TerraformErrorf(err, err.Error(), "(Data) ibm_cr_quota", "read")
		log.Printf("[DEBUG]\n%s", tfErr.GetDebugMessage())
		return tfErr.GetDiag()
	}

	quota, response, err := containerRegistryClient.GetQuotaWithContext(context, &containerregistryv1.GetQuotaOptions{})
	if err != nil {
		tfErr := flex.TerraformErrorf(err, fmt.Sprintf("GetQuotaWithContext failed: %s\n%s", err.Error(), response), "(Data) ibm_cr_quota", "read")
		log.Printf("[DEBUG]\n%s", tfErr.GetDebugMessage())
		return tfErr.GetDiag()
	}
	if quota == nil {
		tfErr := flex.TerraformErrorf(fmt.Errorf("the registry returned no quota"), "GetQuotaWithContext returned no quota", "(Data) ibm_cr_quota", "read")
		log.Printf("[DEBUG]\n%s", tfErr.GetDebugMessage())
		return tfErr.GetDiag()
	}

	d.SetId(crRegistryHost(containerRegistryClient))

	if quota.Limit != nil {
		if quota.Limit.StorageBytes != nil {
			if err = d.Set("storage_megabytes", crQuotaBytesToMegabytes(*quota.Limit.StorageBytes)); err != nil {
				return flex.DiscriminatedTerraformErrorf(err, fmt.Sprintf("Error setting storage_megabytes: %s", err), "(Data) ibm_cr_quota", "read", "set-storage_megabytes").GetDiag()
			}
		}
		if quota.Limit.TrafficBytes != nil {
			if err = d.Set("traffic_megabytes", crQuotaBytesToMegabytes(*quota.Limit.TrafficBytes)); err != nil {
				return flex.DiscriminatedTerraformErrorf(err, fmt.Sprintf("Error setting traffic_megabytes: %s", err), "(Data) ibm_cr_quota", "read", "set-traffic_megabytes").GetDiag()
			}
		}
	}
	if err = d.Set("limit", dataSourceIBMCrQuotaDetailsToMap(quota.Limit)); err != nil {
		return flex.DiscriminatedTerraformErrorf(err, fmt.Sprintf("Error setting limit: %s", err), "(Data) ibm_cr_quota", "read", "set-limit").GetDiag()
	}
	if err = d.Set("usage", dataSourceIBMCrQuotaDetailsToMap(quota.Usage)); err != nil {
		return flex.DiscriminatedTerraformErrorf(err, fmt.Sprintf("Error setting usage: %s", err), "(Data) ibm_cr_quota", "read", "set-usage").GetDiag()
	}
	if err = d.Set("registry", d.Id()); err != nil {
		return flex.DiscriminatedTerraformErrorf(err, fmt.Sprintf("Error setting registry: %s", err), "(Data) ibm_cr_quota", "read", "set-registry").GetDiag()
	}

	return nil
}

func dataSourceIBMCrQuotaDetailsToMap(details *containerregistryv1.QuotaDetails) []map[string]interface{} {
	if details == nil {
		return []map[string]interface{}{}
	}
	detailsMap := map[string]interface{}{}
	if details.StorageBytes != nil {
		detailsMap["storage_bytes"] = flex.IntValue(details.StorageBytes)
	}
	if details.TrafficBytes != nil {
		detailsMap["traffic_bytes"] = flex.IntValue(details.TrafficBytes)
	}
	return []map[string]interface{}{detailsMap}
}
