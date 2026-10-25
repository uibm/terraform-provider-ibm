// Copyright IBM Corp. 2026 All Rights Reserved.
// Licensed under the Mozilla Public License v2.0

package logsrouting

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/IBM-Cloud/terraform-provider-ibm/ibm/conns"
	"github.com/IBM-Cloud/terraform-provider-ibm/ibm/flex"
	"github.com/IBM-Cloud/terraform-provider-ibm/ibm/validate"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const serviceTenantResourceName = "ibm_logs_router_service_tenant"

// ResourceIBMLogsRouterServiceTenant manages an IBM Cloud Logs Routing
// service-level tenant: a tenant created under an account-level tenant for an
// onboarding IBM Cloud service, enabled for supertenancy. ibm_logs_router_tenant
// only covers account-level tenants (POST /tenants).
func ResourceIBMLogsRouterServiceTenant() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceIBMLogsRouterServiceTenantCreate,
		ReadContext:   resourceIBMLogsRouterServiceTenantRead,
		UpdateContext: resourceIBMLogsRouterServiceTenantUpdate,
		DeleteContext: resourceIBMLogsRouterServiceTenantDelete,
		Importer: &schema.ResourceImporter{
			State: func(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				parts := strings.Split(d.Id(), "/")
				if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
					return nil, fmt.Errorf("Invalid import format: please specify 'tenant_id/region'")
				}
				d.SetId(parts[0])
				if err := d.Set("region", parts[1]); err != nil {
					return nil, err
				}
				return []*schema.ResourceData{d}, nil
			},
		},

		Schema: map[string]*schema.Schema{
			"account_level_tenant_id": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "ID of the parent account-level tenant, for example ibm_logs_router_tenant.x.id.",
			},
			"service_name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Name of the IBM Cloud service that is onboarding to Logs Routing.",
			},
			"region": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validate.InvokeValidator(serviceTenantResourceName, "region"),
				Description:  "Logs Routing region of the service-level tenant. It can differ from the region of the Cloud Logs instance.",
			},
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validate.InvokeValidator(serviceTenantResourceName, "name"),
				Description:  "Name of the service-level tenant. 1 to 35 characters: letters, digits, '-' and '.'; must start and end with a letter or digit.",
			},
			"targets": {
				Type:        schema.TypeList,
				Required:    true,
				MinItems:    1,
				MaxItems:    1,
				Description: "Target that receives the tenant's logs. One IBM Cloud Logs target.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id":           {Type: schema.TypeString, Computed: true, Description: "Unique ID of the target."},
						"log_sink_crn": {Type: schema.TypeString, Required: true, ForceNew: true, Description: "CRN of the IBM Cloud Logs instance."},
						"name": {
							Type: schema.TypeString, Required: true,
							ValidateFunc: validate.InvokeValidator(serviceTenantResourceName, "name"),
							Description:  "Name of the target. Same rules as the tenant name.",
						},
						"etag":       {Type: schema.TypeString, Computed: true, Description: "Resource version identifier."},
						"type":       {Type: schema.TypeString, Computed: true, Description: "Type of log-sink, for example logs."},
						"created_at": {Type: schema.TypeString, Computed: true, Description: "Time stamp the target was originally created."},
						"updated_at": {Type: schema.TypeString, Computed: true, Description: "Time stamp the target was last updated."},
						"parameters": {
							Type:        schema.TypeList,
							Required:    true,
							MaxItems:    1,
							Description: "Connection parameters of the Cloud Logs ingestion endpoint.",
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"host": {Type: schema.TypeString, Required: true, Description: "Cloud Logs ingestion host."},
									"port": {Type: schema.TypeInt, Optional: true, Default: 3443, Description: "Ingestion port. Default: 3443."},
								},
							},
						},
					},
				},
			},
			"created_at": {Type: schema.TypeString, Computed: true, Description: "Time stamp the tenant was originally created."},
			"updated_at": {Type: schema.TypeString, Computed: true, Description: "Time stamp the tenant was last updated."},
			"crn":        {Type: schema.TypeString, Computed: true, Description: "Cloud resource name of the tenant."},
			"etag":       {Type: schema.TypeString, Computed: true, Description: "Resource version identifier."},
			"write_status": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "The status of the write attempt to the target with the provided endpoint parameters.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"status":                  {Type: schema.TypeString, Computed: true, Description: "The status such as failed or success."},
						"reason_for_last_failure": {Type: schema.TypeString, Computed: true, Description: "Detailed description of the cause of the failure."},
						"last_failure":            {Type: schema.TypeString, Computed: true, Description: "The timestamp of the failure."},
					},
				},
			},
		},
	}
}

// ResourceIBMLogsRouterServiceTenantValidator validates names like the Logs
// Routing API: 1 to 35 characters of [a-zA-Z0-9-.], starting and ending with a
// letter or digit (the API rejects names ending with a non-alphanumeric).
func ResourceIBMLogsRouterServiceTenantValidator() *validate.ResourceValidator {
	validateSchema := []validate.ValidateSchema{
		{
			Identifier:                 "name",
			ValidateFunctionIdentifier: validate.ValidateRegexpLen,
			Type:                       validate.TypeString,
			Required:                   true,
			Regexp:                     `^[a-zA-Z0-9](?:[a-zA-Z0-9.-]*[a-zA-Z0-9])?$`,
			MinValueLength:             1,
			MaxValueLength:             35,
		},
		{
			Identifier:                 "region",
			ValidateFunctionIdentifier: validate.ValidateRegexpLen,
			Type:                       validate.TypeString,
			Required:                   true,
			Regexp:                     `^[a-zA-Z0-9 -._:]+$`,
			MinValueLength:             3,
			MaxValueLength:             1000,
		},
	}
	return &validate.ResourceValidator{ResourceName: serviceTenantResourceName, Schema: validateSchema}
}

func serviceTenantClientFor(d *schema.ResourceData, meta interface{}) (*serviceTenantClient, error) {
	session := meta.(conns.ClientSession)
	client, err := session.IBMCloudLogsRoutingV0()
	if err != nil {
		return nil, err
	}
	bxSession, err := session.BluemixSession()
	if err != nil {
		return nil, err
	}
	client, _, err = updateClientURLWithEndpoint(client, d, bxSession)
	if err != nil {
		return nil, err
	}
	sc := &serviceTenantClient{client: client}
	if user, err := session.BluemixUserDetails(); err == nil && user != nil {
		sc.accountID = user.UserAccount
	}
	return sc, nil
}

func expandServiceTenantTarget(d *schema.ResourceData) serviceTenantTargetPrototype {
	t := serviceTenantTargetPrototype{
		LogSinkCRN: d.Get("targets.0.log_sink_crn").(string),
		Name:       d.Get("targets.0.name").(string),
		Parameters: serviceTenantTargetParameters{
			Host: d.Get("targets.0.parameters.0.host").(string),
			Port: int64(d.Get("targets.0.parameters.0.port").(int)),
		},
	}
	return t
}

func resourceIBMLogsRouterServiceTenantCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c, err := serviceTenantClientFor(d, meta)
	if err != nil {
		return flex.DiscriminatedTerraformErrorf(err, err.Error(), serviceTenantResourceName, "create", "initialize-client").GetDiag()
	}
	body := &serviceTenantPrototype{
		Name:        d.Get("name").(string),
		ServiceName: d.Get("service_name").(string),
		Targets:     []serviceTenantTargetPrototype{expandServiceTenantTarget(d)},
	}
	tenant, _, err := c.create(ctx, d.Get("account_level_tenant_id").(string), body)
	if err != nil {
		return flex.TerraformErrorf(err, fmt.Sprintf("Creating the service-level tenant failed: %s", err), serviceTenantResourceName, "create").GetDiag()
	}
	if tenant == nil || tenant.ID == nil || tenant.ID.String() == "" {
		err = fmt.Errorf("the create response did not include a tenant id")
		return flex.TerraformErrorf(err, err.Error(), serviceTenantResourceName, "create").GetDiag()
	}
	d.SetId(tenant.ID.String())
	return resourceIBMLogsRouterServiceTenantRead(ctx, d, meta)
}

func resourceIBMLogsRouterServiceTenantRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c, err := serviceTenantClientFor(d, meta)
	if err != nil {
		return flex.DiscriminatedTerraformErrorf(err, err.Error(), serviceTenantResourceName, "read", "initialize-client").GetDiag()
	}
	tenant, response, err := c.get(ctx, d.Id())
	if err != nil {
		if isLogsRoutingNotFound(response) {
			log.Printf("[WARN] service-level tenant %s not found, removing it from state", d.Id())
			d.SetId("")
			return nil
		}
		return flex.TerraformErrorf(err, fmt.Sprintf("Reading the service-level tenant failed: %s", err), serviceTenantResourceName, "read").GetDiag()
	}
	if tenant == nil {
		d.SetId("")
		return nil
	}

	targets := []map[string]interface{}{}
	for _, item := range tenant.Targets {
		m, err := ResourceIBMLogsRouterTenantTargetTypeToMap(item)
		if err != nil {
			return flex.DiscriminatedTerraformErrorf(err, err.Error(), serviceTenantResourceName, "read", "targets-to-map").GetDiag()
		}
		targets = append(targets, m)
	}
	writeStatus := []map[string]interface{}{}
	if tenant.WriteStatus != nil {
		ws, err := ResourceIBMLogsRouterTenantWriteStatusToMap(tenant.WriteStatus)
		if err != nil {
			return flex.DiscriminatedTerraformErrorf(err, err.Error(), serviceTenantResourceName, "read", "write_status-to-map").GetDiag()
		}
		writeStatus = append(writeStatus, ws)
	}
	for k, v := range map[string]interface{}{
		"name": tenant.Name, "targets": targets, "created_at": tenant.CreatedAt, "updated_at": tenant.UpdatedAt,
		"crn": tenant.CRN, "etag": tenant.Etag, "write_status": writeStatus,
	} {
		if err := d.Set(k, v); err != nil {
			err = fmt.Errorf("Error setting %s: %s", k, err)
			return flex.DiscriminatedTerraformErrorf(err, err.Error(), serviceTenantResourceName, "read", "set-"+k).GetDiag()
		}
	}
	return nil
}

func resourceIBMLogsRouterServiceTenantUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c, err := serviceTenantClientFor(d, meta)
	if err != nil {
		return flex.DiscriminatedTerraformErrorf(err, err.Error(), serviceTenantResourceName, "update", "initialize-client").GetDiag()
	}
	if d.HasChange("name") {
		if _, err := c.updateName(ctx, d.Id(), d.Get("etag").(string), d.Get("name").(string)); err != nil {
			return flex.TerraformErrorf(err, fmt.Sprintf("Updating the service-level tenant name failed: %s", err), serviceTenantResourceName, "update").GetDiag()
		}
	}
	if d.HasChange("targets.0.name") || d.HasChange("targets.0.parameters") {
		target := expandServiceTenantTarget(d)
		body := map[string]interface{}{
			"name":       target.Name,
			"parameters": map[string]interface{}{"host": target.Parameters.Host, "port": target.Parameters.Port},
		}
		if _, err := c.updateTarget(ctx, d.Id(), d.Get("targets.0.id").(string), d.Get("targets.0.etag").(string), body); err != nil {
			return flex.TerraformErrorf(err, fmt.Sprintf("Updating the service-level tenant target failed: %s", err), serviceTenantResourceName, "update").GetDiag()
		}
	}
	return resourceIBMLogsRouterServiceTenantRead(ctx, d, meta)
}

func resourceIBMLogsRouterServiceTenantDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c, err := serviceTenantClientFor(d, meta)
	if err != nil {
		return flex.DiscriminatedTerraformErrorf(err, err.Error(), serviceTenantResourceName, "delete", "initialize-client").GetDiag()
	}
	if response, err := c.delete(ctx, d.Id()); err != nil && !isLogsRoutingNotFound(response) {
		return flex.TerraformErrorf(err, fmt.Sprintf("Deleting the service-level tenant failed: %s", err), serviceTenantResourceName, "delete").GetDiag()
	}
	d.SetId("")
	return nil
}
