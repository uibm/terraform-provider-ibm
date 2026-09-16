---
layout: "ibm"
page_title: "IBM : ibm_cr_plan"
description: |-
  Manages the pricing plan of IBM Cloud Container Registry.
subcategory: "Container Registry"
---

# ibm_cr_plan

Manage the IBM Cloud Container Registry pricing plan of your account in the targeted registry. For more information, about Container Registry plans, see [Service plans](https://cloud.ibm.com/docs/Registry?topic=Registry-registry_overview#registry_plans) and [Upgrading your service plan](https://cloud.ibm.com/docs/Registry?topic=Registry-registry_overview#registry_plan_upgrade).

The plan is an account-wide setting that is scoped to a single registry instance (one of the regional registries or the global registry). The registry that is targeted is derived from the `region` of the provider configuration, in the same way as for the `ibm_cr_namespace` resource. To manage the plan in more than one registry, use one provider configuration per region.

~> **Note:** The plan always exists for an account and cannot be deleted. Destroying this resource only removes it from the Terraform state; the plan of the registry is left unchanged. Upgrading to the `Standard` plan is a billing-relevant operation, and the registry may reject a change back to the `Free` plan. Your IBM Cloud account must be a Pay-As-You-Go or Subscription account to upgrade to the `Standard` plan.

## Example usage

```terraform
resource "ibm_cr_plan" "cr_plan" {
  plan = "Standard"
}
```

Manage the plan in a specific registry by targeting its region.

```terraform
provider "ibm" {
  alias  = "eu"
  region = "eu-de"
}

resource "ibm_cr_plan" "cr_plan_eu" {
  provider = ibm.eu
  plan     = "Standard"
}
```

## Argument reference

Review the argument references that you can specify for your resource.

- `plan` - (Required, String) The pricing plan of the registry for your account. Allowed values are `Standard` and `Free`. The value is compared case-insensitively, so `standard` and `Standard` denote the same plan. Applying the plan that the registry already reports does not issue any change.

## Attribute reference

In addition to all argument reference list, you can access the following attribute reference after your resource is created.

- `id` - (String) The unique identifier of the cr_plan. This identifier is the host name of the targeted registry, for example `us.icr.io`.
- `registry` - (String) The host name of the registry instance to which the plan applies, for example `us.icr.io`.

## Import

You can import the `ibm_cr_plan` resource by using the host name of the targeted registry as the identifier. The identifier is only informational, because the registry is determined by the provider configuration.

```
$ terraform import ibm_cr_plan.cr_plan us.icr.io
```
