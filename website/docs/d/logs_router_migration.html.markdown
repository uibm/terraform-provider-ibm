---
layout: "ibm"
page_title: "IBM : ibm_logs_router_migration"
description: |-
  Reads the IBM Cloud Logs Routing migration state of the account.
subcategory: "Logs Routing API Version 3"
---

# ibm_logs_router_migration

Reads the Logs Routing migration state of the account (`GET /migrate`). Use it to check the state before you create V3 targets and routes.

## Example Usage

```hcl
data "ibm_logs_router_migration" "status" {}

resource "ibm_logs_router_target" "target" {
  name            = "my-target"
  destination_crn = var.logs_crn
  region          = "us-south"

  lifecycle {
    precondition {
      condition     = data.ibm_logs_router_migration.status.state == "COMPLETE"
      error_message = "The account has not finished the Logs Routing V3 migration."
    }
  }
}
```

## Attribute Reference

* `id` - Always `logs-router-migration`.
* `state` - `BEFORE`, `IN_PROGRESS`, `PENDING_COMPLETION` or `COMPLETE`. A `404` from the API is reported as `COMPLETE`.
* `api_version` - The API version in use for the account.
* `message` - More information about the state.
