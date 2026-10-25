---
layout: "ibm"
page_title: "IBM : ibm_logs_router_service_tenant"
description: |-
  Manages a service-level tenant in IBM Cloud Logs Routing.
subcategory: "IBM Cloud Logs Routing"
---

# ibm_logs_router_service_tenant

Create, update, and delete a service-level tenant in IBM Cloud Logs Routing. A service-level tenant sits under an account-level tenant (`ibm_logs_router_tenant`) and routes the logs of one service to its own target.

The resource calls `POST /v1/tenants/{account_level_tenant_id}/services` to create the tenant, and `/v1/tenants/{id}` to read, update, and delete it. Every request sends the `Bss-Account-Id` header (the account of the provider credentials) and `IBM-API-Version: 2025-01-21`.

## Example Usage

```hcl
resource "ibm_logs_router_tenant" "account" {
  name   = "account-tenant"
  region = "eu-de"
  targets {
    log_sink_crn = ibm_resource_instance.logs.crn
    name         = "account-target"
    parameters {
      host = "${ibm_resource_instance.logs.guid}.ingress.eu-de.logs.cloud.ibm.com"
      port = 443
    }
  }
}

resource "ibm_logs_router_service_tenant" "example" {
  account_level_tenant_id = ibm_logs_router_tenant.account.id
  service_name            = "my-service"
  name                    = "my-service-tenant"
  region                  = "eu-de"
  targets {
    log_sink_crn = ibm_resource_instance.logs.crn
    name         = "my-service-target"
    parameters {
      host = "${ibm_resource_instance.logs.guid}.ingress.eu-de.logs.cloud.ibm.com"
    }
  }
}
```

## Argument Reference

* `account_level_tenant_id` - (Required, Forces new resource, String) The ID of the parent account-level tenant.
* `service_name` - (Required, Forces new resource, String) The name of the service that the tenant routes logs for.
* `name` - (Required, String) The name of the tenant. 1 to 35 characters: letters, digits, `.` and `-`. It must start and end with a letter or digit.
* `region` - (Required, Forces new resource, String) The region of the tenant, for example `eu-de`.
* `targets` - (Required, List) Exactly one target.
  * `log_sink_crn` - (Required, Forces new resource, String) The CRN of the IBM Cloud Logs instance.
  * `name` - (Required, String) The name of the target.
  * `parameters` - (Required, List) Connection details.
    * `host` - (Required, String) The ingress host of the log sink.
    * `port` - (Optional, Integer) The port. Default is `3443`.

## Attribute Reference

* `id` - The ID of the service-level tenant.
* `crn` - The CRN of the tenant.
* `etag` - The current entity tag of the tenant.
* `created_at` - The time the tenant was created.
* `updated_at` - The time the tenant was last updated.
* `write_status` - The status of the last write (`status`, `reason_for_last_failure`, `last_failure`).
* `targets` - Also exports `id`, `etag`, `type`, `created_at`, `updated_at` for each target.

## Import

Import a service-level tenant with `<tenant_id>/<region>`. The `account_level_tenant_id` and `service_name` values are not returned by the API, so set them in your configuration after import.

```
$ terraform import ibm_logs_router_service_tenant.example 8717db99-2cfb-4ba6-a033-89c994c2e9f0/eu-de
```
