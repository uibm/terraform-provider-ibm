---
layout: "ibm"
page_title: "IBM : ibm_logs_router_migration"
description: |-
  Migrates an account from IBM Cloud Logs Routing API version 1 to version 3.
subcategory: "Logs Routing API Version 3"
---

# ibm_logs_router_migration

Migrates the account from IBM Cloud Logs Routing API version 1 (tenants) to version 3 (targets and routes). See [Migrating to Logs Routing version 3](https://cloud.ibm.com/docs/logs-router?topic=logs-router-v3-migration).

The migration is one object per account. It moves through these states:

```
BEFORE --generate--> IN_PROGRESS --(automatic)--> PENDING_COMPLETION --complete--> COMPLETE
   ^                                                       |
   +------------------- reset (DELETE /migrate) -----------+
```

The usual flow takes two applies:

1. Apply with `complete = false`. The resource starts the migration (`POST /migrate?action=generate`) and waits until the state is `PENDING_COMPLETION`. Review the generated V3 targets and routes with the `ibm_logs_router_targets` and `ibm_logs_router_routes` data sources.
2. Set `complete = true` and apply again. The resource finishes the migration (`POST /migrate?action=complete`) and the account moves to API version 3.

~> **Completion cannot be undone.** After `complete = true` is applied, the account stays on API version 3. Log delivery is briefly interrupted (a few minutes) while the account switches. Changing `complete` from `true` back to `false` is rejected at plan time.

~> **Reset deletes all V3 targets and routes.** With `reset_on_destroy = true`, `terraform destroy` before completion calls `DELETE /migrate`. This deletes **every** V3 target and route in the account, not only the generated ones, and sets the state back to `BEFORE`. The default is `false`, so destroy never resets unless you ask for it.

Before you start, set `primary_metadata_region` with `ibm_logs_router_settings`. You need the Administrator platform role (IAM actions `logs-router.migration.post`, `logs-router.migration.get`, `logs-router.migration.delete`).

## Example Usage

```hcl
resource "ibm_logs_router_settings" "settings" {
  primary_metadata_region = "us-south"
}

resource "ibm_logs_router_migration" "migration" {
  # Step 1: false, review the generated targets and routes.
  # Step 2: true, finish the migration (cannot be undone).
  complete = false

  depends_on = [ibm_logs_router_settings.settings]
}

data "ibm_logs_router_targets" "generated" {
  depends_on = [ibm_logs_router_migration.migration]
}

data "ibm_logs_router_routes" "generated" {
  depends_on = [ibm_logs_router_migration.migration]
}
```

## Argument Reference

* `complete` - (Optional, Bool) Set to `true` to finish the migration once the state is `PENDING_COMPLETION`. Completion cannot be undone, and log delivery is briefly interrupted. Default is `false`. Changing it from `true` to `false` fails at plan time.
* `reset_on_destroy` - (Optional, Bool) When `true`, destroy in state `BEFORE` or `PENDING_COMPLETION` resets the migration and **deletes all V3 targets and routes in the account**. When `false` (default), destroy only removes the resource from Terraform state. The value in state is what destroy uses, so apply a change to it before you destroy.

## Attribute Reference

* `id` - Always `logs-router-migration`. The migration has no ID of its own.
* `state` - `BEFORE`, `IN_PROGRESS`, `PENDING_COMPLETION` or `COMPLETE`.
* `api_version` - The API version in use for the account.
* `message` - More information about the state.

## Behavior

* **Create:** In `BEFORE`, starts the migration and waits for `PENDING_COMPLETION`. In `IN_PROGRESS`, waits. In `PENDING_COMPLETION` or `COMPLETE`, adopts the current state without calling generate. Then completes the migration if `complete = true`.
* **Destroy:** In `IN_PROGRESS`, first waits for the state to change. In `COMPLETE`, removes the resource from state with a warning; nothing is undone. In `BEFORE` or `PENDING_COMPLETION`, resets only when `reset_on_destroy = true`. A `405` from the reset API is reported as a clear error.
* **Accounts that started on version 3:** if `GET /migrate` returns `404`, the account is treated as `COMPLETE` on API version 3.

## Timeouts

* `create` - (Default 30 minutes) Waiting for generate (and complete when `complete = true`).
* `update` - (Default 30 minutes) Waiting for complete.
* `delete` - (Default 30 minutes) Waiting for an `IN_PROGRESS` migration before reset.

## Import

Import uses the fixed ID `logs-router-migration`. If the account is already `COMPLETE`, import sets `complete = true`, so set `complete = true` in your configuration to get a plan with no diff.

```
$ terraform import ibm_logs_router_migration.migration logs-router-migration
```
