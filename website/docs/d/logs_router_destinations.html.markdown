---
layout: "ibm"
page_title: "IBM : ibm_logs_router_destinations"
description: |-
  Shows where IBM Cloud Logs Routing sends the platform logs of resources.
subcategory: "Logs Routing API Version 3"
---

# ibm_logs_router_destinations

Queries where the platform logs of the given resources are sent (`POST /destinations/query`).

The API returns a map keyed by CRN. This data source returns it as a list, one entry per requested CRN in request order, with the CRN as a field.

## Example Usage

```hcl
data "ibm_logs_router_destinations" "query" {
  crns = [ibm_is_vpc.vpc.crn]
}

output "destinations" {
  value = data.ibm_logs_router_destinations.query.destinations[0].items
}
```

## Argument Reference

* `crns` - (Required, List of String) CRNs of the resources to query.

## Attribute Reference

* `destinations` - (List) One entry per requested CRN.
  * `crn` - (String) The queried resource CRN.
  * `items` - (List) Where the logs of the resource go. Empty when the API returned nothing for the CRN.
    * `account` - (String) Account that owns the destination.
    * `action` - (String) `send`, or `drop` when the logs are explicitly dropped.
    * `crn` - (String) CRN of the destination.
    * `region` - (String) Region of the destination.
    * `service_name` - (String) CRN service name of the destination.
    * `associated_targets` - (List) Targets that point to the destination.
      * `crn`, `id`, `name` - (String) The target.
      * `association_reasons` - (List) Why the target is associated: `type`, and the route `name` and `crn` when applicable.
