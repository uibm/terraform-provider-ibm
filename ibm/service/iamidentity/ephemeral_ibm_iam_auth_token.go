// Copyright IBM Corp. 2026 All Rights Reserved.
// Licensed under the Mozilla Public License v2.0

package iamidentity

import (
	"context"
	"fmt"
	"strings"

	"github.com/IBM-Cloud/terraform-provider-ibm/ibm/conns"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ ephemeral.EphemeralResource              = &iamAuthTokenEphemeral{}
	_ ephemeral.EphemeralResourceWithConfigure = &iamAuthTokenEphemeral{}
)

// NewIAMAuthTokenEphemeralResource is the ephemeral twin of data.ibm_iam_auth_token.
// Every Mastercard/restapi provider block in terraform-ibm-modules authenticates
// with that data source, which writes a live bearer token to plan and state. The
// ephemeral version is never persisted and can feed provider blocks and
// write-only arguments directly.
func NewIAMAuthTokenEphemeralResource() ephemeral.EphemeralResource {
	return &iamAuthTokenEphemeral{}
}

type iamAuthTokenEphemeral struct {
	session conns.ClientSession
}

type iamAuthTokenModel struct {
	IAMAccessToken  types.String `tfsdk:"iam_access_token"`
	IAMRefreshToken types.String `tfsdk:"iam_refresh_token"`
	AccountID       types.String `tfsdk:"account_id"`
}

func (e *iamAuthTokenEphemeral) Metadata(ctx context.Context, req ephemeral.MetadataRequest, resp *ephemeral.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_auth_token"
}

func (e *iamAuthTokenEphemeral) Schema(ctx context.Context, req ephemeral.SchemaRequest, resp *ephemeral.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Returns the IAM access token of the provider session without storing it in plan or state.",
		Attributes: map[string]schema.Attribute{
			"iam_access_token": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "IAM access token, including the 'Bearer ' prefix, ready for an Authorization header.",
			},
			"iam_refresh_token": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "IAM refresh token of the session, when one exists.",
			},
			"account_id": schema.StringAttribute{
				Computed:    true,
				Description: "Account ID the token belongs to.",
			},
		},
	}
}

func (e *iamAuthTokenEphemeral) Configure(ctx context.Context, req ephemeral.ConfigureRequest, resp *ephemeral.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	session, ok := req.ProviderData.(conns.ClientSession)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Provider Data Type",
			fmt.Sprintf("Expected conns.ClientSession, got: %T. Please report this issue to the provider developers.", req.ProviderData))
		return
	}
	e.session = session
}

func (e *iamAuthTokenEphemeral) Open(ctx context.Context, req ephemeral.OpenRequest, resp *ephemeral.OpenResponse) {
	if e.session == nil {
		resp.Diagnostics.AddError("Provider Not Configured", "The IBM provider session is not available.")
		return
	}
	bmxSess, err := e.session.BluemixSession()
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read IAM Session", err.Error())
		return
	}

	token := bmxSess.Config.IAMAccessToken
	if token != "" && !strings.HasPrefix(token, "Bearer ") {
		token = "Bearer " + token
	}
	data := iamAuthTokenModel{
		IAMAccessToken:  types.StringValue(token),
		IAMRefreshToken: types.StringValue(bmxSess.Config.IAMRefreshToken),
		AccountID:       types.StringNull(),
	}
	if user, err := e.session.BluemixUserDetails(); err == nil && user != nil {
		data.AccountID = types.StringValue(user.UserAccount)
	}
	resp.Diagnostics.Append(resp.Result.Set(ctx, &data)...)
}
