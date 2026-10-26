// Copyright IBM Corp. 2026 All Rights Reserved.
// Licensed under the Mozilla Public License v2.0

package logsrouter

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/IBM-Cloud/terraform-provider-ibm/ibm/conns"
	"github.com/IBM-Cloud/terraform-provider-ibm/ibm/flex"
	"github.com/IBM/platform-services-go-sdk/logsrouterv3"
)

func DataSourceIBMLogsRouterDestinations() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceIBMLogsRouterDestinationsRead,

		Schema: map[string]*schema.Schema{
			"crns": {
				Type:        schema.TypeList,
				Required:    true,
				MinItems:    1,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "CRNs of the resources to query.",
			},
			"destinations": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "One entry per requested CRN, in request order.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"crn": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The queried resource CRN.",
						},
						"items": {
							Type:        schema.TypeList,
							Computed:    true,
							Description: "Where the platform logs of the resource go.",
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"account":      {Type: schema.TypeString, Computed: true, Description: "Account that owns the destination."},
									"action":       {Type: schema.TypeString, Computed: true, Description: "send, or drop when the logs are explicitly dropped."},
									"crn":          {Type: schema.TypeString, Computed: true, Description: "CRN of the destination."},
									"region":       {Type: schema.TypeString, Computed: true, Description: "Region of the destination."},
									"service_name": {Type: schema.TypeString, Computed: true, Description: "CRN service name of the destination."},
									"associated_targets": {
										Type:        schema.TypeList,
										Computed:    true,
										Description: "Targets that point to the destination.",
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"crn":  {Type: schema.TypeString, Computed: true, Description: "CRN of the target."},
												"id":   {Type: schema.TypeString, Computed: true, Description: "ID of the target."},
												"name": {Type: schema.TypeString, Computed: true, Description: "Name of the target."},
												"association_reasons": {
													Type:        schema.TypeList,
													Computed:    true,
													Description: "Why the target is associated with the resource.",
													Elem: &schema.Resource{
														Schema: map[string]*schema.Schema{
															"type": {Type: schema.TypeString, Computed: true, Description: "Type of association reason."},
															"name": {Type: schema.TypeString, Computed: true, Description: "Name of the route, if applicable."},
															"crn":  {Type: schema.TypeString, Computed: true, Description: "CRN of the route, if applicable."},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func dataSourceIBMLogsRouterDestinationsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client, err := meta.(conns.ClientSession).LogsRouterV3()
	if err != nil {
		tfErr := flex.DiscriminatedTerraformErrorf(err, err.Error(), "(Data) ibm_logs_router_destinations", "read", "initialize-client")
		log.Printf("[DEBUG]\n%s", tfErr.GetDebugMessage())
		return tfErr.GetDiag()
	}

	crns := flex.ExpandStringList(d.Get("crns").([]interface{}))
	prototypes := make([]logsrouterv3.CRNPrototype, 0, len(crns))
	for i := range crns {
		prototypes = append(prototypes, logsrouterv3.CRNPrototype{CRN: &crns[i]})
	}
	result, _, err := client.QueryDestinationsWithContext(ctx, client.NewQueryDestinationsOptions(prototypes))
	if err != nil {
		tfErr := flex.DiscriminatedTerraformErrorf(err, err.Error(), "(Data) ibm_logs_router_destinations", "read", "query")
		log.Printf("[DEBUG]\n%s", tfErr.GetDebugMessage())
		return tfErr.GetDiag()
	}

	d.SetId(fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(crns, ",")))))
	if err := d.Set("destinations", flattenDestinationsQuery(crns, result)); err != nil {
		return flex.DiscriminatedTerraformErrorf(err, err.Error(), "(Data) ibm_logs_router_destinations", "read", "set-destinations").GetDiag()
	}
	return nil
}

// flattenDestinationsQuery turns the map keyed by CRN into a list in request order,
// because a Terraform schema cannot key nested objects by arbitrary strings.
func flattenDestinationsQuery(crns []string, result *logsrouterv3.DestinationsQuery) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(crns))
	for _, crn := range crns {
		items := []map[string]interface{}{}
		if result != nil {
			for _, dest := range result.Destinations[crn] {
				targets := []map[string]interface{}{}
				for _, t := range dest.AssociatedTargets {
					reasons := []map[string]interface{}{}
					for _, r := range t.AssociationReasons {
						reasons = append(reasons, map[string]interface{}{
							"type": flex.StringValue(r.Type), "name": flex.StringValue(r.Name), "crn": flex.StringValue(r.CRN),
						})
					}
					targets = append(targets, map[string]interface{}{
						"crn": flex.StringValue(t.CRN), "id": flex.StringValue(t.ID), "name": flex.StringValue(t.Name), "association_reasons": reasons,
					})
				}
				items = append(items, map[string]interface{}{
					"account": flex.StringValue(dest.Account), "action": flex.StringValue(dest.Action), "crn": flex.StringValue(dest.CRN),
					"region": flex.StringValue(dest.Region), "service_name": flex.StringValue(dest.ServiceName), "associated_targets": targets,
				})
			}
		}
		out = append(out, map[string]interface{}{"crn": crn, "items": items})
	}
	return out
}
