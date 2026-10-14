// Copyright IBM Corp. 2026 All Rights Reserved.
// Licensed under the Mozilla Public License v2.0

package provider_framework

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// TestFrameworkSchemasValid checks that every framework construct registered by
// the provider returns a schema that passes the framework's own validation, and
// that type names are unique per construct kind.
func TestFrameworkSchemasValid(t *testing.T) {
	ctx := context.Background()
	p := &frameworkProvider{version: "test"}

	seen := map[string]bool{}
	for _, f := range p.Resources(ctx) {
		r := f()
		md := &resource.MetadataResponse{}
		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "ibm"}, md)
		sr := &resource.SchemaResponse{}
		r.Schema(ctx, resource.SchemaRequest{}, sr)
		if sr.Diagnostics.HasError() {
			t.Fatalf("resource %s schema: %v", md.TypeName, sr.Diagnostics)
		}
		if d := sr.Schema.ValidateImplementation(ctx); d.HasError() {
			t.Fatalf("resource %s schema invalid: %v", md.TypeName, d)
		}
		if seen["r/"+md.TypeName] {
			t.Fatalf("duplicate resource %s", md.TypeName)
		}
		seen["r/"+md.TypeName] = true
	}
	for _, f := range p.DataSources(ctx) {
		d := f()
		md := &datasource.MetadataResponse{}
		d.Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: "ibm"}, md)
		sr := &datasource.SchemaResponse{}
		d.Schema(ctx, datasource.SchemaRequest{}, sr)
		if diags := sr.Schema.ValidateImplementation(ctx); diags.HasError() {
			t.Fatalf("data source %s schema invalid: %v", md.TypeName, diags)
		}
	}
	for _, f := range p.EphemeralResources(ctx) {
		e := f()
		md := &ephemeral.MetadataResponse{}
		e.Metadata(ctx, ephemeral.MetadataRequest{ProviderTypeName: "ibm"}, md)
		sr := &ephemeral.SchemaResponse{}
		e.Schema(ctx, ephemeral.SchemaRequest{}, sr)
		if diags := sr.Schema.ValidateImplementation(ctx); diags.HasError() {
			t.Fatalf("ephemeral %s schema invalid: %v", md.TypeName, diags)
		}
	}
	for _, f := range p.Actions(ctx) {
		a := f()
		md := &action.MetadataResponse{}
		a.Metadata(ctx, action.MetadataRequest{ProviderTypeName: "ibm"}, md)
		sr := &action.SchemaResponse{}
		a.Schema(ctx, action.SchemaRequest{}, sr)
		if diags := sr.Schema.ValidateImplementation(ctx); diags.HasError() {
			t.Fatalf("action %s schema invalid: %v", md.TypeName, diags)
		}
	}

	for _, want := range []string{"r/ibm_cr_quota", "r/ibm_resource_instance_parameters", "r/ibm_watsonx_storage_delegation", "r/ibm_watsonx_project"} {
		if !seen[want] {
			t.Errorf("expected %s to be registered", want)
		}
	}
}
