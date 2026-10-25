// Copyright IBM Corp. 2026 All Rights Reserved.
// Licensed under the Mozilla Public License v2.0

package secretsmanager

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/secrets-manager-go-sdk/v2/secretsmanagerv2"
	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestRotateDefaults(t *testing.T) {
	pub, _ := secretsmanagerv2.NewSecretsManagerV2(&secretsmanagerv2.SecretsManagerV2Options{
		URL: "https://secrets-manager.eu-de.appdomain.cloud", Authenticator: &core.NoAuthAuthenticator{},
	})
	priv, _ := secretsmanagerv2.NewSecretsManagerV2(&secretsmanagerv2.SecretsManagerV2Options{
		URL: "https://private.secrets-manager.us-south.appdomain.cloud", Authenticator: &core.NoAuthAuthenticator{},
	})
	if r := regionOrDefault(pub, types.StringNull()); r != "eu-de" {
		t.Errorf("region from public URL = %q", r)
	}
	if r := regionOrDefault(priv, types.StringNull()); r != "us-south" {
		t.Errorf("region from private URL = %q", r)
	}
	if r := regionOrDefault(pub, types.StringValue("jp-tok")); r != "jp-tok" {
		t.Errorf("configured region = %q", r)
	}
	if e := endpointTypeOrDefault(pub, types.StringNull()); e != "public" {
		t.Errorf("endpoint type public = %q", e)
	}
	if e := endpointTypeOrDefault(priv, types.StringNull()); e != "private" {
		t.Errorf("endpoint type private = %q", e)
	}
	if _, err := parseVersionMetadata(`{"source":"terraform"}`); err != nil {
		t.Error(err)
	}
	if _, err := parseVersionMetadata(`[1]`); err == nil {
		t.Error("expected error for a non-object")
	}
}

// TestRotateRequest checks the call the action makes: a new arbitrary secret
// version with the payload, on POST /api/v2/secrets/{id}/versions.
func TestRotateRequest(t *testing.T) {
	var gotMethod, gotPath string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"secret_type":"arbitrary","id":"v2","secret_id":"s1","created_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer srv.Close()
	client, err := secretsmanagerv2.NewSecretsManagerV2(&secretsmanagerv2.SecretsManagerV2Options{URL: srv.URL, Authenticator: &core.NoAuthAuthenticator{}})
	if err != nil {
		t.Fatal(err)
	}
	proto, err := client.NewArbitrarySecretVersionPrototype("new-api-key")
	if err != nil {
		t.Fatal(err)
	}
	proto.VersionCustomMetadata = map[string]interface{}{"source": "terraform"}
	version, _, err := client.CreateSecretVersionWithContext(context.Background(), client.NewCreateSecretVersionOptions("s1", proto))
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost || gotPath != "/api/v2/secrets/s1/versions" {
		t.Errorf("method=%s path=%s", gotMethod, gotPath)
	}
	if body["payload"] != "new-api-key" || body["version_custom_metadata"].(map[string]any)["source"] != "terraform" {
		t.Errorf("body = %v", body)
	}
	if v, ok := version.(*secretsmanagerv2.ArbitrarySecretVersion); !ok || *v.ID != "v2" {
		t.Errorf("version = %#v", version)
	}
}

func TestRotateSchemaValid(t *testing.T) {
	ctx := context.Background()
	sr := &action.SchemaResponse{}
	NewArbitrarySecretRotateAction().Schema(ctx, action.SchemaRequest{}, sr)
	if d := sr.Schema.ValidateImplementation(ctx); d.HasError() {
		t.Errorf("schema invalid: %v", d)
	}
}
