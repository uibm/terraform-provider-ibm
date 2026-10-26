// Copyright IBM Corp. 2026 All Rights Reserved.
// Licensed under the Mozilla Public License v2.0

package logsrouter

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/IBM-Cloud/terraform-provider-ibm/ibm/conns"
	"github.com/IBM-Cloud/terraform-provider-ibm/ibm/flex"
	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/platform-services-go-sdk/logsrouterv3"
)

const (
	migrationResourceName = "ibm_logs_router_migration"
	// The migration is a single account-level object with no ID of its own.
	migrationID = "logs-router-migration"

	migrationStateBefore            = logsrouterv3.MigrationStateStateBeforeConst
	migrationStateInProgress        = logsrouterv3.MigrationStateStateInProgressConst
	migrationStatePendingCompletion = logsrouterv3.MigrationStateStatePendingCompletionConst
	migrationStateComplete          = logsrouterv3.MigrationStateStateCompleteConst

	migrationPollInterval = 15 * time.Second
)

func ResourceIBMLogsRouterMigration() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceIBMLogsRouterMigrationCreate,
		ReadContext:   resourceIBMLogsRouterMigrationRead,
		UpdateContext: resourceIBMLogsRouterMigrationUpdate,
		DeleteContext: resourceIBMLogsRouterMigrationDelete,
		Importer: &schema.ResourceImporter{
			StateContext: resourceIBMLogsRouterMigrationImport,
		},
		CustomizeDiff: resourceIBMLogsRouterMigrationCustomizeDiff,

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(30 * time.Minute),
			Update: schema.DefaultTimeout(30 * time.Minute),
			Delete: schema.DefaultTimeout(30 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"complete": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
				Description: "Set to true to finish the migration once the generated V3 targets and routes are reviewed (state PENDING_COMPLETION). " +
					"Completion cannot be undone, and log delivery is briefly interrupted while the account switches to API version 3.",
			},
			"reset_on_destroy": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
				Description: "When true, destroy in state BEFORE or PENDING_COMPLETION calls DELETE /migrate, which deletes ALL V3 targets and routes in the account " +
					"(not only the generated ones) and sets the state back to BEFORE. When false (default), destroy only removes the resource from state.",
			},
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

// Completion cannot be undone, so turning complete off is rejected at plan time.
func resourceIBMLogsRouterMigrationCustomizeDiff(_ context.Context, d *schema.ResourceDiff, _ interface{}) error {
	if d.Id() == "" || !d.HasChange("complete") {
		return nil
	}
	if o, n := d.GetChange("complete"); o.(bool) && !n.(bool) {
		return fmt.Errorf("complete cannot be changed from true to false: a completed Logs Routing migration cannot be undone")
	}
	return nil
}

func resourceIBMLogsRouterMigrationCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client, err := meta.(conns.ClientSession).LogsRouterV3()
	if err != nil {
		return migrationError(err, "create", "initialize-client")
	}

	state, err := getMigrationState(ctx, client)
	if err != nil {
		return migrationError(err, "create", "get-status")
	}
	switch *state.State {
	case migrationStateBefore:
		log.Printf("[INFO] Starting Logs Routing migration (action=generate)")
		if _, _, err := client.MigrateActionsWithContext(ctx, client.NewMigrateActionsOptions(logsrouterv3.MigrateActionsOptionsActionGenerateConst)); err != nil {
			return migrationError(err, "create", "generate")
		}
		fallthrough
	case migrationStateInProgress:
		if state, err = waitForMigration(ctx, client, d.Timeout(schema.TimeoutCreate)); err != nil {
			return migrationError(err, "create", "wait-generate")
		}
	}
	// PENDING_COMPLETION and COMPLETE are adopted as they are.
	d.SetId(migrationID)

	if d.Get("complete").(bool) && *state.State == migrationStatePendingCompletion {
		if _, err := completeMigration(ctx, client, d.Timeout(schema.TimeoutCreate)); err != nil {
			return migrationError(err, "create", "complete")
		}
	}
	return resourceIBMLogsRouterMigrationRead(ctx, d, meta)
}

func resourceIBMLogsRouterMigrationRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client, err := meta.(conns.ClientSession).LogsRouterV3()
	if err != nil {
		return migrationError(err, "read", "initialize-client")
	}
	state, err := getMigrationState(ctx, client)
	if err != nil {
		return migrationError(err, "read", "get-status")
	}
	if err := setMigrationState(d, state); err != nil {
		return migrationError(err, "read", "set-state")
	}
	return nil
}

func resourceIBMLogsRouterMigrationUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	if !d.HasChange("complete") || !d.Get("complete").(bool) {
		// reset_on_destroy only changes what destroy does.
		return resourceIBMLogsRouterMigrationRead(ctx, d, meta)
	}
	client, err := meta.(conns.ClientSession).LogsRouterV3()
	if err != nil {
		return migrationError(err, "update", "initialize-client")
	}
	state, err := getMigrationState(ctx, client)
	if err != nil {
		return migrationError(err, "update", "get-status")
	}
	if *state.State == migrationStateInProgress {
		if state, err = waitForMigration(ctx, client, d.Timeout(schema.TimeoutUpdate)); err != nil {
			return migrationError(err, "update", "wait-generate")
		}
	}
	switch *state.State {
	case migrationStatePendingCompletion:
		if _, err := completeMigration(ctx, client, d.Timeout(schema.TimeoutUpdate)); err != nil {
			return migrationError(err, "update", "complete")
		}
	case migrationStateBefore:
		return migrationError(fmt.Errorf("the migration is in state BEFORE (it was reset outside Terraform), so it cannot be completed. "+
			"Recreate this resource to generate the V3 configuration again"), "update", "complete")
	}
	return resourceIBMLogsRouterMigrationRead(ctx, d, meta)
}

func resourceIBMLogsRouterMigrationDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client, err := meta.(conns.ClientSession).LogsRouterV3()
	if err != nil {
		return migrationError(err, "delete", "initialize-client")
	}
	state, err := getMigrationState(ctx, client)
	if err != nil {
		return migrationError(err, "delete", "get-status")
	}
	if *state.State == migrationStateInProgress {
		if state, err = waitForMigration(ctx, client, d.Timeout(schema.TimeoutDelete)); err != nil {
			return migrationError(err, "delete", "wait-generate")
		}
	}

	switch {
	case *state.State == migrationStateComplete:
		return diag.Diagnostics{{
			Severity: diag.Warning,
			Summary:  "Logs Routing migration is complete and cannot be undone",
			Detail:   "ibm_logs_router_migration was removed from Terraform state only. The account stays on API version 3.",
		}}
	case !d.Get("reset_on_destroy").(bool):
		return diag.Diagnostics{{
			Severity: diag.Warning,
			Summary:  "Logs Routing migration was not reset",
			Detail: fmt.Sprintf("ibm_logs_router_migration was removed from Terraform state only; the migration stays in state %s. "+
				"Set reset_on_destroy = true and apply before destroy to reset it.", *state.State),
		}}
	case *state.State == migrationStateBefore:
		return nil
	}

	if err := resetMigration(ctx, client); err != nil {
		return migrationError(err, "delete", "reset")
	}
	return nil
}

// resetMigration calls DELETE /migrate and explains the 405 the API returns
// in states IN_PROGRESS and COMPLETE.
func resetMigration(ctx context.Context, client *logsrouterv3.LogsRouterV3) error {
	log.Printf("[INFO] Resetting Logs Routing migration (DELETE /migrate)")
	_, response, err := client.ResetMigrationWithContext(ctx, client.NewResetMigrationOptions())
	if err != nil && response != nil && response.StatusCode == http.StatusMethodNotAllowed {
		return fmt.Errorf("the migration cannot be reset in its current state (reset is allowed only in BEFORE or PENDING_COMPLETION): %w", err)
	}
	return err
}

// Import adopts the account's migration; complete follows the current state so
// that a following plan has no diff for an account that is already COMPLETE.
func resourceIBMLogsRouterMigrationImport(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	client, err := meta.(conns.ClientSession).LogsRouterV3()
	if err != nil {
		return nil, err
	}
	state, err := getMigrationState(ctx, client)
	if err != nil {
		return nil, err
	}
	d.SetId(migrationID)
	if err := d.Set("complete", *state.State == migrationStateComplete); err != nil {
		return nil, err
	}
	if err := d.Set("reset_on_destroy", false); err != nil {
		return nil, err
	}
	return []*schema.ResourceData{d}, nil
}

// getMigrationState reads GET /migrate. An account that started on V3 may have
// no migration record; a 404 is treated as COMPLETE on API version 3.
func getMigrationState(ctx context.Context, client *logsrouterv3.LogsRouterV3) (*logsrouterv3.MigrationState, error) {
	state, response, err := client.GetMigrationStatusWithContext(ctx, client.NewGetMigrationStatusOptions())
	if err != nil {
		if response != nil && response.StatusCode == http.StatusNotFound {
			return &logsrouterv3.MigrationState{
				APIVersion: core.Int64Ptr(3),
				State:      core.StringPtr(migrationStateComplete),
				Message:    core.StringPtr("No migration record found; the account uses API version 3."),
			}, nil
		}
		return nil, err
	}
	if state == nil || state.State == nil {
		return nil, fmt.Errorf("GET /migrate returned no state")
	}
	return state, nil
}

// waitForMigration polls until the state leaves IN_PROGRESS (and BEFORE, which can
// be reported briefly right after generate is accepted).
func waitForMigration(ctx context.Context, client *logsrouterv3.LogsRouterV3, timeout time.Duration) (*logsrouterv3.MigrationState, error) {
	return waitForMigrationState(ctx, client, timeout, migrationPollInterval,
		[]string{migrationStateBefore, migrationStateInProgress},
		[]string{migrationStatePendingCompletion, migrationStateComplete})
}

func completeMigration(ctx context.Context, client *logsrouterv3.LogsRouterV3, timeout time.Duration) (*logsrouterv3.MigrationState, error) {
	log.Printf("[INFO] Completing Logs Routing migration (action=complete)")
	if _, _, err := client.MigrateActionsWithContext(ctx, client.NewMigrateActionsOptions(logsrouterv3.MigrateActionsOptionsActionCompleteConst)); err != nil {
		return nil, err
	}
	return waitForMigrationState(ctx, client, timeout, migrationPollInterval,
		[]string{migrationStatePendingCompletion}, []string{migrationStateComplete})
}

func waitForMigrationState(ctx context.Context, client *logsrouterv3.LogsRouterV3, timeout, interval time.Duration, pending, target []string) (*logsrouterv3.MigrationState, error) {
	var last *logsrouterv3.MigrationState
	conf := &retry.StateChangeConf{
		Pending:      pending,
		Target:       target,
		Timeout:      timeout,
		PollInterval: interval,
		Refresh: func() (interface{}, string, error) {
			state, err := getMigrationState(ctx, client)
			if err != nil {
				return nil, "", err
			}
			last = state
			return state, *state.State, nil
		},
	}
	result, err := conf.WaitForStateContext(ctx)
	if err != nil {
		if last != nil && last.Message != nil && *last.Message != "" {
			err = fmt.Errorf("%w (last state %s: %s)", err, *last.State, *last.Message)
		}
		return nil, err
	}
	return result.(*logsrouterv3.MigrationState), nil
}

func setMigrationState(d *schema.ResourceData, state *logsrouterv3.MigrationState) error {
	if err := d.Set("state", state.State); err != nil {
		return err
	}
	if err := d.Set("api_version", flex.IntValue(state.APIVersion)); err != nil {
		return err
	}
	return d.Set("message", state.Message)
}

func migrationError(err error, operation, discriminator string) diag.Diagnostics {
	tfErr := flex.DiscriminatedTerraformErrorf(err, err.Error(), migrationResourceName, operation, discriminator)
	log.Printf("[DEBUG]\n%s", tfErr.GetDebugMessage())
	return tfErr.GetDiag()
}
