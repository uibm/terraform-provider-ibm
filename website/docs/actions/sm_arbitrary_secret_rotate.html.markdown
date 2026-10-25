---
subcategory: "Secrets Manager"
layout: "ibm"
page_title: "IBM : ibm_sm_arbitrary_secret_rotate"
description: |-
  Rotates an existing arbitrary secret with a new payload.
---

# ibm_sm_arbitrary_secret_rotate (Action)

Rotates an existing arbitrary secret by creating a new secret version with the given payload (`POST /api/v2/secrets/{id}/versions`). Terraform does not manage or delete the secret itself. The action config, including the payload, is not stored in state.

It replaces `restapi_object.update_secret_payload` in the IAM User API Key workflow, which called the legacy `/api/v1/secrets/arbitrary/{id}?action=rotate`.

## Example usage

Create an API key for a user and store it in an existing secret:

```terraform
data "ibm_iam_user_profile" "user" {
  iam_id = var.user_email
}

resource "ibm_iam_api_key" "user_key" {
  name   = var.api_key_name
  iam_id = data.ibm_iam_user_profile.user.ibm_id
}

action "ibm_sm_arbitrary_secret_rotate" "user_key" {
  config {
    instance_id = var.secrets_manager_guid
    region      = var.region
    secret_id   = var.secret_id
    payload     = ibm_iam_api_key.user_key.apikey
  }
}

# Rotate whenever a new API key is created.
resource "terraform_data" "rotate" {
  input = ibm_iam_api_key.user_key.apikey_id

  lifecycle {
    action_trigger {
      events  = [after_create, after_update]
      actions = [action.ibm_sm_arbitrary_secret_rotate.user_key]
    }
  }
}
```

To create a new secret instead, use `ibm_sm_arbitrary_secret` with `payload = ibm_iam_api_key.user_key.apikey`.

## Argument reference

- `instance_id` - (Required, String) GUID of the Secrets Manager instance.
- `region` - (Optional, String) Region of the instance. Defaults to the provider's region.
- `endpoint_type` - (Optional, String) `public` or `private`. Defaults to the provider's visibility.
- `secret_id` - (Required, String) ID of the arbitrary secret to rotate.
- `payload` - (Required, String) New secret value.
- `version_custom_metadata` - (Optional, String) JSON object of custom metadata for the new version.
