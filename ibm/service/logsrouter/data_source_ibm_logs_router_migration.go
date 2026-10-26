// Copyright IBM Corp. 2026 All Rights Reserved.
// Licensed under the Mozilla Public License v2.0

package logsrouter

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/IBM-Cloud/terraform-provider-ibm/ibm/conns"
	"github.com/IBM-Cloud/terraform-provider-ibm/ibm/flex"
)

func DataSourceIBMLogsRouterMigration() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceIBMLogsRouterMigrationRead,

		Schema: map[string]*schema.Schema{
			"state": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Migration state: BEFORE, IN_PROGRESS, PENDING_COMPLETION or COMPLETE.",
			},
			"api_version": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "API version currently in use for IBM Cloud Logs Routing in the account.",
			},
			"message": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Additional information about the migration state.",
			},
		},
	}
}

func dataSourceIBMLogsRouterMigrationRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client, err := meta.(conns.ClientSession).LogsRouterV3()
	if err != nil {
		tfErr := flex.DiscriminatedTerraformErrorf(err, err.Error(), "(Data) ibm_logs_router_migration", "read", "initialize-client")
		log.Printf("[DEBUG]\n%s", tfErr.GetDebugMessage())
		return tfErr.GetDiag()
	}
	state, err := getMigrationState(ctx, client)
	if err != nil {
		tfErr := flex.DiscriminatedTerraformErrorf(err, err.Error(), "(Data) ibm_logs_router_migration", "read", "get-status")
		log.Printf("[DEBUG]\n%s", tfErr.GetDebugMessage())
		return tfErr.GetDiag()
	}
	d.SetId(migrationID)
	if err := setMigrationState(d, state); err != nil {
		return flex.DiscriminatedTerraformErrorf(err, err.Error(), "(Data) ibm_logs_router_migration", "read", "set-state").GetDiag()
	}
	return nil
}
