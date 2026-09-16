---
subcategory: "Container Registry"
layout: "ibm"
page_title: "IBM: ibm_cr_quota"
description: |-
  Reads the storage and pull traffic quotas and usage of IBM Cloud Container Registry.
---
# ibm_cr_quota

Retrieve the IBM Cloud Container Registry storage and pull traffic quotas of your account in the targeted registry, together with the current usage against those quotas. For more information about Container Registry quotas, see [Quota limits and billing](https://cloud.ibm.com/docs/Registry?topic=Registry-registry_overview#registry_plan_billing).

The registry that is targeted is derived from the `region` of the provider configuration, in the same way as for the `ibm_cr_namespaces` data source.

## Example usage

The following example retrieves the quotas and usage for your account in the targeted registry.

```terraform
data "ibm_cr_quota" "cr_quota" {}

output "storage_usage_bytes" {
  value = data.ibm_cr_quota.cr_quota.usage[0].storage_bytes
}
```

## Argument reference

Input parameters are not supported for this data source.

## Attribute reference

Review the attribute references that are exported.

- `id` - (String) The unique identifier of the ibm_cr_quota data source. This identifier is the host name of the targeted registry, for example `us.icr.io`.
- `storage_megabytes` - (Integer) The storage quota in megabytes. The value `-1` denotes `Unlimited`. This is the value that the `storage_megabytes` argument of the `ibm_cr_quota` resource accepts.
- `traffic_megabytes` - (Integer) The pull traffic quota in megabytes per month. The value `-1` denotes `Unlimited`. This is the value that the `traffic_megabytes` argument of the `ibm_cr_quota` resource accepts.
- `limit` - (List) The quota limits of the registry for your account.

  Nested scheme for `limit`:
  - `storage_bytes` - (Integer) The storage quota in bytes. The value `-1` denotes `Unlimited`.
  - `traffic_bytes` - (Integer) The pull traffic quota in bytes per month. The value `-1` denotes `Unlimited`.
- `usage` - (List) The current usage of the registry for your account.

  Nested scheme for `usage`:
  - `storage_bytes` - (Integer) The storage that is currently used, in bytes.
  - `traffic_bytes` - (Integer) The pull traffic that was used in the current month, in bytes.
- `registry` - (String) The host name of the registry instance to which the quotas apply, for example `us.icr.io`.
