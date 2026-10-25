// Copyright IBM Corp. 2026 All Rights Reserved.
// Licensed under the Mozilla Public License v2.0

package logsrouting

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/logs-router-go-sdk/ibmcloudlogsroutingv0"
)

// serviceTenantAPIVersion is the IBM-API-Version sent on every service-level
// tenant call. The /tenants/{id}/services create call is not in the public API
// reference or in IBM/logs-router-go-sdk (v1.0.8 pins 2024-06-15 and does not
// let callers override the header), and the terraform-ibm-modules workaround
// that is known to work sends 2025-01-21 on all calls, so this resource sends
// its own requests with that version.
const serviceTenantAPIVersion = "2025-01-21"

// serviceTenantClient sends service-level tenant requests through the
// provider's Logs Routing client (endpoint already resolved for the region and
// visibility), so it reuses its authenticator, retries and URL.
type serviceTenantClient struct {
	client    *ibmcloudlogsroutingv0.IBMCloudLogsRoutingV0
	accountID string
}

type serviceTenantTargetPrototype struct {
	LogSinkCRN string                        `json:"log_sink_crn"`
	Name       string                        `json:"name"`
	Parameters serviceTenantTargetParameters `json:"parameters"`
}

type serviceTenantTargetParameters struct {
	Host string `json:"host"`
	Port int64  `json:"port"`
}

type serviceTenantPrototype struct {
	Name        string                         `json:"name"`
	ServiceName string                         `json:"service_name"`
	Targets     []serviceTenantTargetPrototype `json:"targets"`
}

// create sends POST /tenants/{account_level_tenant_id}/services.
func (c *serviceTenantClient) create(ctx context.Context, accountLevelTenantID string, body *serviceTenantPrototype) (*ibmcloudlogsroutingv0.Tenant, *core.DetailedResponse, error) {
	return c.tenantCall(ctx, core.POST, "/tenants/{tenant_id}/services", map[string]string{"tenant_id": accountLevelTenantID}, "application/json", "", body)
}

// get sends GET /tenants/{tenant_id}.
func (c *serviceTenantClient) get(ctx context.Context, tenantID string) (*ibmcloudlogsroutingv0.Tenant, *core.DetailedResponse, error) {
	return c.tenantCall(ctx, core.GET, "/tenants/{tenant_id}", map[string]string{"tenant_id": tenantID}, "", "", nil)
}

// updateName sends PATCH /tenants/{tenant_id} with the stored etag, as the SDK's
// UpdateTenant does.
func (c *serviceTenantClient) updateName(ctx context.Context, tenantID, etag, name string) (*core.DetailedResponse, error) {
	_, resp, err := c.tenantCall(ctx, core.PATCH, "/tenants/{tenant_id}", map[string]string{"tenant_id": tenantID}, "application/merge-patch+json", etag, map[string]string{"name": name})
	return resp, err
}

// updateTarget sends PATCH /tenants/{tenant_id}/targets/{target_id} with the
// target's etag, as the SDK's UpdateTarget does.
func (c *serviceTenantClient) updateTarget(ctx context.Context, tenantID, targetID, etag string, body map[string]interface{}) (*core.DetailedResponse, error) {
	return c.do(ctx, core.PATCH, "/tenants/{tenant_id}/targets/{target_id}",
		map[string]string{"tenant_id": tenantID, "target_id": targetID}, "application/merge-patch+json", etag, body, nil)
}

// delete sends DELETE /tenants/{tenant_id}.
func (c *serviceTenantClient) delete(ctx context.Context, tenantID string) (*core.DetailedResponse, error) {
	return c.do(ctx, core.DELETE, "/tenants/{tenant_id}", map[string]string{"tenant_id": tenantID}, "", "", nil, nil)
}

func (c *serviceTenantClient) tenantCall(ctx context.Context, method, path string, params map[string]string, contentType, ifMatch string, body any) (*ibmcloudlogsroutingv0.Tenant, *core.DetailedResponse, error) {
	var raw map[string]json.RawMessage
	resp, err := c.do(ctx, method, path, params, contentType, ifMatch, body, &raw)
	if err != nil || raw == nil {
		return nil, resp, err
	}
	var tenant *ibmcloudlogsroutingv0.Tenant
	if err := core.UnmarshalModel(raw, "", &tenant, ibmcloudlogsroutingv0.UnmarshalTenant); err != nil {
		return nil, resp, err
	}
	return tenant, resp, nil
}

func (c *serviceTenantClient) do(ctx context.Context, method, path string, params map[string]string, contentType, ifMatch string, body, out any) (*core.DetailedResponse, error) {
	builder := core.NewRequestBuilder(method).WithContext(ctx)
	builder.EnableGzipCompression = c.client.GetEnableGzipCompression()
	if _, err := builder.ResolveRequestURL(c.client.Service.Options.URL, path, params); err != nil {
		return nil, err
	}
	builder.AddHeader("Accept", "application/json")
	builder.AddHeader("IBM-API-Version", serviceTenantAPIVersion)
	if c.accountID != "" {
		// Sent by the known-working workaround; not in the SDK or public docs.
		builder.AddHeader("Bss-Account-Id", c.accountID)
	}
	if ifMatch != "" {
		builder.AddHeader("If-Match", ifMatch)
	}
	if body != nil {
		builder.AddHeader("Content-Type", contentType)
		if _, err := builder.SetBodyContentJSON(body); err != nil {
			return nil, err
		}
	}
	request, err := builder.Build()
	if err != nil {
		return nil, err
	}
	if out == nil {
		return c.client.Service.Request(request, nil)
	}
	return c.client.Service.Request(request, out)
}

func isLogsRoutingNotFound(r *core.DetailedResponse) bool {
	return r != nil && r.StatusCode == http.StatusNotFound
}
