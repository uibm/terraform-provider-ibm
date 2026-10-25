// Copyright IBM Corp. 2026 All Rights Reserved.
// Licensed under the Mozilla Public License v2.0

package secretsmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/IBM-Cloud/terraform-provider-ibm/ibm/conns"
	"github.com/IBM/secrets-manager-go-sdk/v2/secretsmanagerv2"
	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ action.Action                   = &arbitrarySecretRotateAction{}
	_ action.ActionWithConfigure      = &arbitrarySecretRotateAction{}
	_ action.ActionWithValidateConfig = &arbitrarySecretRotateAction{}
)

// NewArbitrarySecretRotateAction rotates an existing arbitrary secret by
// creating a new secret version with the given payload
// (POST /api/v2/secrets/{id}/versions).
//
// It replaces restapi_object.update_secret_payload in the IAM User API Key
// workflow, which POSTed the legacy /api/v1/secrets/arbitrary/{id}?action=rotate
// and had a no-op GET destroy. Rotation has nothing to delete, so it is an
// action: the secret is not managed by Terraform, only its payload is replaced.
func NewArbitrarySecretRotateAction() action.Action {
	return &arbitrarySecretRotateAction{}
}

type arbitrarySecretRotateAction struct {
	client        *secretsmanagerv2.SecretsManagerV2
	endpointsFile string
}

type arbitrarySecretRotateModel struct {
	InstanceID            types.String `tfsdk:"instance_id"`
	Region                types.String `tfsdk:"region"`
	EndpointType          types.String `tfsdk:"endpoint_type"`
	SecretID              types.String `tfsdk:"secret_id"`
	Payload               types.String `tfsdk:"payload"`
	VersionCustomMetadata types.String `tfsdk:"version_custom_metadata"`
}

func (a *arbitrarySecretRotateAction) Metadata(ctx context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = "ibm_sm_arbitrary_secret_rotate"
}

func (a *arbitrarySecretRotateAction) Schema(ctx context.Context, req action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Rotates an existing arbitrary secret in Secrets Manager by creating a new secret version with the given payload. The secret itself is not managed or deleted. Action config, including the payload, is not stored in state.",
		Attributes: map[string]schema.Attribute{
			"instance_id": schema.StringAttribute{Required: true, Description: "GUID of the Secrets Manager instance."},
			"region": schema.StringAttribute{
				Optional:    true,
				Description: "Region of the Secrets Manager instance. Defaults to the provider's region.",
			},
			"endpoint_type": schema.StringAttribute{
				Optional:    true,
				Description: "public or private. Defaults to the provider's visibility.",
			},
			"secret_id": schema.StringAttribute{Required: true, Description: "ID of the arbitrary secret to rotate."},
			"payload": schema.StringAttribute{
				Required:    true,
				Description: "New secret value, for example ibm_iam_api_key.user_key.apikey.",
			},
			"version_custom_metadata": schema.StringAttribute{
				Optional:    true,
				Description: "JSON object of custom metadata for the new version.",
			},
		},
	}
}

func (a *arbitrarySecretRotateAction) ValidateConfig(ctx context.Context, req action.ValidateConfigRequest, resp *action.ValidateConfigResponse) {
	var cfg arbitrarySecretRotateModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !cfg.EndpointType.IsNull() && !cfg.EndpointType.IsUnknown() {
		if t := cfg.EndpointType.ValueString(); t != "public" && t != "private" {
			resp.Diagnostics.AddAttributeError(path.Root("endpoint_type"), "Invalid Endpoint Type", "endpoint_type must be public or private.")
		}
	}
	if !cfg.VersionCustomMetadata.IsNull() && !cfg.VersionCustomMetadata.IsUnknown() {
		if _, err := parseVersionMetadata(cfg.VersionCustomMetadata.ValueString()); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("version_custom_metadata"), "Invalid JSON", err.Error())
		}
	}
}

func (a *arbitrarySecretRotateAction) Configure(ctx context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	session, ok := req.ProviderData.(conns.ClientSession)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Provider Data Type",
			fmt.Sprintf("Expected conns.ClientSession, got: %T. Please report this issue to the provider developers.", req.ProviderData))
		return
	}
	client, endpointsFile, err := getSecretsManagerSession(session)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Create Secrets Manager Client", err.Error())
		return
	}
	a.client, a.endpointsFile = client, endpointsFile
}

func (a *arbitrarySecretRotateAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var cfg arbitrarySecretRotateModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if a.client == nil {
		resp.Diagnostics.AddError("Provider Not Configured", "The Secrets Manager client is not available.")
		return
	}
	region := regionOrDefault(a.client, cfg.Region)
	endpointType := endpointTypeOrDefault(a.client, cfg.EndpointType)
	client := getClientWithInstanceEndpoint(a.client, cfg.InstanceID.ValueString(), region, endpointType, a.endpointsFile)

	prototype, err := client.NewArbitrarySecretVersionPrototype(cfg.Payload.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Secret Payload", err.Error())
		return
	}
	if !cfg.VersionCustomMetadata.IsNull() && cfg.VersionCustomMetadata.ValueString() != "" {
		md, err := parseVersionMetadata(cfg.VersionCustomMetadata.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid version_custom_metadata", err.Error())
			return
		}
		prototype.VersionCustomMetadata = md
	}

	secretID := cfg.SecretID.ValueString()
	version, _, err := client.CreateSecretVersionWithContext(ctx, client.NewCreateSecretVersionOptions(secretID, prototype))
	if err != nil {
		resp.Diagnostics.AddError("Unable to Rotate Secret", fmt.Sprintf("Creating a new version of secret %s failed: %s", secretID, err))
		return
	}
	msg := fmt.Sprintf("Secret %s rotated", secretID)
	if v, ok := version.(*secretsmanagerv2.ArbitrarySecretVersion); ok && v.ID != nil {
		msg = fmt.Sprintf("Secret %s rotated to version %s", secretID, *v.ID)
	}
	resp.SendProgress(action.InvokeProgressEvent{Message: msg})
}

// regionOrDefault mirrors getRegion for framework config: the configured
// region, otherwise the region in the provider's Secrets Manager URL
// ("https://<private.>secrets-manager.<region>.<domain>").
func regionOrDefault(client *secretsmanagerv2.SecretsManagerV2, region types.String) string {
	if !region.IsNull() && !region.IsUnknown() && region.ValueString() != "" {
		return region.ValueString()
	}
	u := strings.Replace(client.Service.GetServiceURL(), "private.", "", 1)
	parts := strings.Split(u, ".")
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

// endpointTypeOrDefault mirrors getEndpointType for framework config.
func endpointTypeOrDefault(client *secretsmanagerv2.SecretsManagerV2, endpointType types.String) string {
	if !endpointType.IsNull() && !endpointType.IsUnknown() && endpointType.ValueString() != "" {
		return endpointType.ValueString()
	}
	if strings.Contains(client.Service.GetServiceURL(), "private.") {
		return "private"
	}
	return "public"
}

func parseVersionMetadata(raw string) (map[string]interface{}, error) {
	var md map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &md); err != nil {
		return nil, fmt.Errorf("version_custom_metadata must be a JSON object: %w", err)
	}
	return md, nil
}
