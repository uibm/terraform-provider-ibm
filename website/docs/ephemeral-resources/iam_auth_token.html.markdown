---
subcategory: "IAM Identity Services"
layout: "ibm"
page_title: "IBM : ibm_iam_auth_token"
description: |-
  Ephemeral IAM access token of the provider session.
---

# ibm_iam_auth_token (Ephemeral Resource)

Returns the IAM access token of the provider session without writing it to plan or state. Use it in place of `data.ibm_iam_auth_token` wherever a token only feeds a provider block or a write-only argument. Requires Terraform 1.10 or later.

## Example usage

```terraform
ephemeral "ibm_iam_auth_token" "token" {}

provider "restapi" {
  uri = "https://resource-controller.cloud.ibm.com"
  headers = {
    Authorization = ephemeral.ibm_iam_auth_token.token.iam_access_token
  }
}
```

## Attribute reference

- `iam_access_token` - (Sensitive) Access token with the `Bearer ` prefix.
- `iam_refresh_token` - (Sensitive) Refresh token, when the session has one.
- `account_id` - Account ID of the token.
