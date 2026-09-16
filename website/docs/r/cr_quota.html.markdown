---
layout: "ibm"
page_title: "IBM : ibm_cr_quota"
description: |-
  Manages the storage and pull traffic quotas of IBM Cloud Container Registry.
subcategory: "Container Registry"
---

# ibm_cr_quota

Manage the IBM Cloud Container Registry storage and pull traffic quotas of your account in the targeted registry. For more information, about Container Registry quotas, see [Quota limits and billing](https://cloud.ibm.com/docs/Registry?topic=Registry-registry_overview#registry_plan_billing).

The quotas are account-wide settings that are scoped to a single registry instance (one of the regional registries or the global registry). The registry that is targeted is derived from the `region` of the provider configuration, in the same way as for the `ibm_cr_namespace` resource. To manage the quotas in more than one registry, use one provider configuration per region.

~> **Note:** The quotas always exist for an account and cannot be deleted. Destroying this resource only removes it from the Terraform state; the quotas of the registry are left unchanged. The registry rejects quota values that exceed the limits of your current pricing plan, for example unlimited (`-1`) quotas on the `Free` plan. Use the `ibm_cr_plan` resource to change the plan first.

## Example usage

```terraform
resource "ibm_cr_quota" "cr_quota" {
  storage_megabytes = 500
  traffic_megabytes = 5120
}
```

Upgrade the plan and remove all limits in the same configuration.

```terraform
resource "ibm_cr_plan" "cr_plan" {
  plan = "Standard"
}

resource "ibm_cr_quota" "cr_quota" {
  storage_megabytes = -1
  traffic_megabytes = -1

  depends_on = [ibm_cr_plan.cr_plan]
}
```

Manage only the storage quota and leave the pull traffic quota untouched.

```terraform
resource "ibm_cr_quota" "cr_quota" {
  storage_megabytes = 2048
}
```

## Argument reference

Review the argument references that you can specify for your resource. At least one of `storage_megabytes` and `traffic_megabytes` must be specified.

- `storage_megabytes` - (Optional, Integer) The storage quota in megabytes. The value `-1` denotes `Unlimited`. When omitted, the current storage quota of the registry is kept and exported.
- `traffic_megabytes` - (Optional, Integer) The pull traffic quota in megabytes per month. The value `-1` denotes `Unlimited`. When omitted, the current pull traffic quota of the registry is kept and exported.

## Attribute reference

In addition to all argument reference list, you can access the following attribute reference after your resource is created.

- `id` - (String) The unique identifier of the cr_quota. This identifier is the host name of the targeted registry, for example `us.icr.io`.
- `registry` - (String) The host name of the registry instance to which the quotas apply, for example `us.icr.io`.

## Import

You can import the `ibm_cr_quota` resource by using the host name of the targeted registry as the identifier. The identifier is only informational, because the registry is determined by the provider configuration.

```
$ terraform import ibm_cr_quota.cr_quota us.icr.io
```
