---
subcategory: "Identity & Access Management (IAM)"
layout: "ibm"
page_title: "IBM : ibm_iam_api_key"
sidebar_current: "docs-ibm-resource-iam-api-key"
description: |-
  Manages an IAM API Key.
---

# ibm_iam_api_key

Create, update, and delete IAM API keys with this resource. For more information about IAM API Keys, see [managing user API keys](https://cloud.ibm.com/docs/account?topic=account-userapikey).

## Example Usage

```hcl
resource "ibm_iam_api_key" "iam_api_key_instance" {
  name = "name"
}
```

Create an API key for another user in the account, looked up by email:

```hcl
data "ibm_iam_user_profile" "user" {
  iam_id = "user@example.com" # the user's email or IAM ID
}

resource "ibm_iam_api_key" "user_key" {
  name        = "ci-pipeline-key"
  description = "Key for the CI pipeline"
  iam_id      = data.ibm_iam_user_profile.user.ibm_id
}
```

## Argument Reference

You can specify the following arguments for this resource.

* `account_id` - (Optional, Forces new resource, String) ID of the account that this API key authenticates for. Defaults to the caller's account.
* `apikey` - (Optional, String) The API key value. This property only contains the API key value for the following cases: create an API key, update a service ID API key that stores the API key value as retrievable, or get a service ID API key that stores the API key value as retrievable. All other operations don't return the API key value, for example all user API key related operations, except for create, don't contain the API key value.
* `description` - (Optional, String) The optional description of the API key. The 'description' property is only available if a description was provided during a create of an API key.
* `entity_lock` - (Optional, String) Indicates if the API key is locked for further write operations. False by default.
  * Constraints: The default value is `false`.
* `expires_at` - (Optional, String) Date and time when the API key becomes invalid, ISO 8601 datetime in the format 'yyyy-MM-ddTHH:mm+0000'. **WARNING** An API key will be permanently and irrevocably deleted when both the expires_at and modified_at timestamps are more than ninety (90) days in the past, regardless of the key’s locked status or any other state.
* `file` - (Optional, String) The file name where API key is to be stored.
* `iam_id` - (Optional, Forces new resource, String) The IAM ID that this API key authenticates. Defaults to the caller. Set it to create an API key for another user in the account, for example `data.ibm_iam_user_profile.user.ibm_id`. User API keys cannot be stored, so leave `store_value` unset or `false`.
* `name` - (Required, String) Name of the API key. The name is not checked for uniqueness. Therefore multiple names with the same value can exist. Access is done via the UUID of the API key.
- `store_value` - (Optional, Bool) Use `true` or `false` to set whether the API key value is retrievable in the future by using the `Get` details of an API key request. If you create an API key for a user, you must specify `false` or omit the value. Users cannot store the API key.

## Attribute Reference

After your resource is created, you can read values from the listed arguments and the following attributes.

* `apikey_id` - (String) The unique identifier of the `ibm_iam_api_key`.
* `created_at` - (String) If set contains a date time string of the creation date in ISO format.
* `created_by` - (String) IAM ID of the user or service which created the API key.
* `crn` - (String) Cloud Resource Name of the item. Example Cloud Resource Name: 'crn:v1:bluemix:public:iam-identity:us-south:a/myaccount::apikey:1234-9012-5678'.
* `entity_tag` - (String) Version of the API Key details object. You need to specify this value when updating the API key to avoid stale updates.
* `locked` - (Boolean) The API key cannot be changed if set to true.
* `modified_at` - (String) If set contains a date time string of the last modification date in ISO format.

## Import

You can import the `ibm_iam_api_key` resource by using `apikey_id`. Unique identifier of this API Key.

# Syntax
<pre>
$ terraform import ibm_iam_api_key.iam_api_key &lt;apikey_id&gt;
</pre>
