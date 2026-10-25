// Copyright IBM Corp. 2026 All Rights Reserved.
// Licensed under the Mozilla Public License v2.0

package logsrouting

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/logs-router-go-sdk/ibmcloudlogsroutingv0"
)

const tenantJSON = `{
  "id": "8717db99-2cfb-4ba6-a033-89c994c2e9f0",
  "created_at": "2026-01-01T00:00:00Z",
  "updated_at": "2026-01-01T00:00:00Z",
  "crn": "crn:v1:bluemix:public:logs-router:us-east:a/acct:8717db99-2cfb-4ba6-a033-89c994c2e9f0::",
  "name": "my-service-tenant",
  "etag": "\"tenant-etag\"",
  "targets": [{
    "id": "9ebd5b4f-1a0b-4b9a-8e1a-0c3b2f6d7e11",
    "log_sink_crn": "crn:v1:bluemix:public:logs:us-east:a/acct:abc::",
    "name": "my-target",
    "etag": "\"target-etag\"",
    "type": "logs",
    "created_at": "2026-01-01T00:00:00Z",
    "updated_at": "2026-01-01T00:00:00Z",
    "parameters": {"host": "abc.ingress.us-east.logs.cloud.ibm.com", "port": 3443}
  }],
  "write_status": {"status": "success"}
}`

type capturedRequest struct {
	method, path, apiVersion, account, contentType, ifMatch string
	body                                                    map[string]any
}

func serviceTenantTestClient(t *testing.T, status int, response string, got *capturedRequest) *serviceTenantClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*got = capturedRequest{
			method: r.Method, path: r.URL.Path, apiVersion: r.Header.Get("IBM-API-Version"),
			account: r.Header.Get("Bss-Account-Id"), contentType: r.Header.Get("Content-Type"), ifMatch: r.Header.Get("If-Match"),
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got.body)
		if response != "" {
			w.Header().Set("Content-Type", "application/json")
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(srv.Close)
	client, err := ibmcloudlogsroutingv0.NewIBMCloudLogsRoutingV0(&ibmcloudlogsroutingv0.IBMCloudLogsRoutingV0Options{
		URL: srv.URL + "/v1", Authenticator: &core.NoAuthAuthenticator{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return &serviceTenantClient{client: client, accountID: "acct-123"}
}

func TestServiceTenantCreate(t *testing.T) {
	var got capturedRequest
	c := serviceTenantTestClient(t, http.StatusCreated, tenantJSON, &got)
	tenant, _, err := c.create(context.Background(), "parent-tenant", &serviceTenantPrototype{
		Name: "my-service-tenant", ServiceName: "my-service",
		Targets: []serviceTenantTargetPrototype{{
			LogSinkCRN: "crn:v1:bluemix:public:logs:us-east:a/acct:abc::", Name: "my-target",
			Parameters: serviceTenantTargetParameters{Host: "abc.ingress.us-east.logs.cloud.ibm.com", Port: 3443},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.method != http.MethodPost || got.path != "/v1/tenants/parent-tenant/services" {
		t.Errorf("method=%s path=%s", got.method, got.path)
	}
	if got.apiVersion != serviceTenantAPIVersion || got.account != "acct-123" || got.contentType != "application/json" {
		t.Errorf("headers: version=%q account=%q content-type=%q", got.apiVersion, got.account, got.contentType)
	}
	targets := got.body["targets"].([]any)
	target := targets[0].(map[string]any)
	if got.body["service_name"] != "my-service" || got.body["name"] != "my-service-tenant" || len(targets) != 1 ||
		target["parameters"].(map[string]any)["port"] != float64(3443) {
		t.Errorf("body = %v", got.body)
	}
	if tenant.ID.String() != "8717db99-2cfb-4ba6-a033-89c994c2e9f0" || *tenant.Etag != `"tenant-etag"` || len(tenant.Targets) != 1 {
		t.Errorf("tenant = %+v", tenant)
	}
	m, err := ResourceIBMLogsRouterTenantTargetTypeToMap(tenant.Targets[0])
	if err != nil || m["type"] != "logs" || m["id"] == nil {
		t.Errorf("target map = %v, %v", m, err)
	}
}

func TestServiceTenantGetAndNotFound(t *testing.T) {
	var got capturedRequest
	c := serviceTenantTestClient(t, http.StatusOK, tenantJSON, &got)
	tenant, _, err := c.get(context.Background(), "t1")
	if err != nil || *tenant.Name != "my-service-tenant" || got.method != http.MethodGet || got.path != "/v1/tenants/t1" || got.apiVersion != serviceTenantAPIVersion {
		t.Fatalf("get: %+v %v %+v", tenant, err, got)
	}
	c = serviceTenantTestClient(t, http.StatusNotFound, `{"errors":[{"code":"not_found"}]}`, &got)
	if _, resp, err := c.get(context.Background(), "gone"); err == nil || !isLogsRoutingNotFound(resp) {
		t.Errorf("want 404, got resp=%v err=%v", resp, err)
	}
}

func TestServiceTenantUpdatesUseEtagAndMergePatch(t *testing.T) {
	var got capturedRequest
	c := serviceTenantTestClient(t, http.StatusOK, tenantJSON, &got)
	if _, err := c.updateName(context.Background(), "t1", `"tenant-etag"`, "renamed"); err != nil {
		t.Fatal(err)
	}
	if got.method != http.MethodPatch || got.path != "/v1/tenants/t1" || got.contentType != "application/merge-patch+json" ||
		got.ifMatch != `"tenant-etag"` || got.body["name"] != "renamed" {
		t.Errorf("updateName request = %+v", got)
	}
	if _, err := c.updateTarget(context.Background(), "t1", "tg1", `"target-etag"`, map[string]interface{}{"name": "new"}); err != nil {
		t.Fatal(err)
	}
	if got.path != "/v1/tenants/t1/targets/tg1" || got.ifMatch != `"target-etag"` || got.contentType != "application/merge-patch+json" {
		t.Errorf("updateTarget request = %+v", got)
	}
}

func TestServiceTenantDelete(t *testing.T) {
	var got capturedRequest
	c := serviceTenantTestClient(t, http.StatusNoContent, "", &got)
	if _, err := c.delete(context.Background(), "t1"); err != nil {
		t.Fatal(err)
	}
	if got.method != http.MethodDelete || got.path != "/v1/tenants/t1" || got.apiVersion != serviceTenantAPIVersion || got.account != "acct-123" {
		t.Errorf("delete request = %+v", got)
	}
}

func TestServiceTenantNameRule(t *testing.T) {
	var re *regexp.Regexp
	for _, s := range ResourceIBMLogsRouterServiceTenantValidator().Schema {
		if s.Identifier == "name" {
			re = regexp.MustCompile(s.Regexp)
		}
	}
	for name, want := range map[string]bool{
		"a": true, "my-service.tenant-1": true, "tenant-": false, "tenant.": false, "-tenant": false, "bad_name": false, "has space": false,
	} {
		if re.MatchString(name) != want {
			t.Errorf("name %q: match=%v want %v", name, !want, want)
		}
	}
}

func TestServiceTenantSchema(t *testing.T) {
	if err := ResourceIBMLogsRouterServiceTenant().InternalValidate(nil, true); err != nil {
		t.Fatal(err)
	}
}
