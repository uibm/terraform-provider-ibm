---
subcategory: "Container Registry"
layout: "ibm"
page_title: "IBM: ibm_cr_plan"
description: |-
  Reads the pricing plan of IBM Cloud Container Registry.
---
# ibm_cr_plan

Retrieve the IBM Cloud Container Registry pricing plan of your account in the targeted registry. For more information about Container Registry plans, see [Service plans](https://cloud.ibm.com/docs/Registry?topic=Registry-registry_overview#registry_plans).

The registry that is targeted is derived from the `region` of the provider configuration, in the same way as for the `ibm_cr_namespaces` data source.

## Example usage

The following example retrieves the plan for your account in the targeted registry.

```terraform
data "ibm_cr_plan" "cr_plan" {}
```

## Argument reference

Input parameters are not supported for this data source.

## Attribute reference

Review the attribute references that are exported.

- `id` - (String) The unique identifier of the ibm_cr_plan data source. This identifier is the host name of the targeted registry, for example `us.icr.io`.
- `plan` - (String) The pricing plan of the registry for your account, for example `Standard` or `Free`.
- `registry` - (String) The host name of the registry instance to which the plan applies, for example `us.icr.io`.
