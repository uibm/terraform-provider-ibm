// Copyright IBM Corp. 2017, 2026 All Rights Reserved.
// Licensed under the Mozilla Public License v2.0

package kubernetes

import (
	"fmt"
	"strings"

	v2 "github.com/IBM-Cloud/bluemix-go/api/container/containerv2"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const (
	allWorkersReady          = "AllWorkersReady"
	workerProvisionPending   = "provision_pending"
	workerHealthFailed       = "failed"
	workerLifecycleFailed    = "deploy_failed"
	defaultVpcWorkerPoolName = "default"
)

func vpcClusterWaitTillValues() []string {
	return []string{masterNodeReady, oneWorkerNodeReady, ingressReady, clusterNormal, allWorkersReady}
}

func vpcWorkerPoolWorkersSchema() *schema.Schema {
	return &schema.Schema{
		Type:        schema.TypeList,
		Computed:    true,
		Description: "Workers that currently belong to this worker pool",
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"id": {
					Type:        schema.TypeString,
					Computed:    true,
					Description: "ID of the worker",
				},
				"state": {
					Type:        schema.TypeString,
					Computed:    true,
					Description: "Health state of the worker",
				},
				"pool_id": {
					Type:        schema.TypeString,
					Computed:    true,
					Description: "ID of the worker pool",
				},
				"pool_name": {
					Type:        schema.TypeString,
					Computed:    true,
					Description: "Name of the worker pool",
				},
				"flavor": {
					Type:        schema.TypeString,
					Computed:    true,
					Description: "Flavor of the worker",
				},
				"kube_version": {
					Type:        schema.TypeString,
					Computed:    true,
					Description: "Actual Kubernetes version of the worker",
				},
				"location": {
					Type:        schema.TypeString,
					Computed:    true,
					Description: "Zone or location of the worker",
				},
				"lifecycle_actual_state": {
					Type:        schema.TypeString,
					Computed:    true,
					Description: "Actual lifecycle state of the worker",
				},
			},
		},
	}
}

func filterWorkersByPool(workers []v2.Worker, poolNameOrID string) []v2.Worker {
	if workers == nil {
		return []v2.Worker{}
	}

	filtered := make([]v2.Worker, 0, len(workers))
	seen := make(map[string]struct{}, len(workers))
	for _, worker := range workers {
		if poolNameOrID != "" && worker.PoolName != poolNameOrID && worker.PoolID != poolNameOrID {
			continue
		}
		if worker.ID != "" {
			if _, exists := seen[worker.ID]; exists {
				continue
			}
			seen[worker.ID] = struct{}{}
		}
		filtered = append(filtered, worker)
	}
	return filtered
}

func flattenVpcWorkerPoolWorkers(workers []v2.Worker) []map[string]interface{} {
	if workers == nil {
		return []map[string]interface{}{}
	}

	result := make([]map[string]interface{}, 0, len(workers))
	for _, worker := range workers {
		result = append(result, map[string]interface{}{
			"id":                     worker.ID,
			"state":                  worker.Health.State,
			"pool_id":                worker.PoolID,
			"pool_name":              worker.PoolName,
			"flavor":                 worker.Flavor,
			"kube_version":           worker.KubeVersion.Actual,
			"location":               worker.Location,
			"lifecycle_actual_state": worker.LifeCycle.ActualState,
		})
	}
	return result
}

func nestVpcWorkerPoolWorkers(flattened []map[string]interface{}, workers []v2.Worker) []map[string]interface{} {
	if flattened == nil {
		return []map[string]interface{}{}
	}

	for i, pool := range flattened {
		poolID, _ := pool["id"].(string)
		poolName, _ := pool["name"].(string)
		flattened[i]["workers"] = flattenVpcWorkerPoolWorkers(workersForPool(workers, poolID, poolName))
	}
	return flattened
}

func mergeWorkersByID(left, right []v2.Worker) []v2.Worker {
	merged := make([]v2.Worker, 0, len(left)+len(right))
	seen := make(map[string]struct{}, len(left)+len(right))
	for _, worker := range append(left, right...) {
		if worker.ID != "" {
			if _, exists := seen[worker.ID]; exists {
				continue
			}
			seen[worker.ID] = struct{}{}
		}
		merged = append(merged, worker)
	}
	return merged
}

func workerHealthOrLifecycle(worker v2.Worker) (string, string) {
	return strings.ToLower(strings.TrimSpace(worker.Health.State)), strings.ToLower(strings.TrimSpace(worker.LifeCycle.ActualState))
}

func workerIsFailedOrCritical(worker v2.Worker) bool {
	health, actual := workerHealthOrLifecycle(worker)
	switch health {
	case clusterCritical, workerHealthFailed:
		return true
	}
	switch actual {
	case workerLifecycleFailed, workerHealthFailed:
		return true
	}
	return false
}

func workerIsReady(worker v2.Worker) bool {
	health, actual := workerHealthOrLifecycle(worker)
	return health == normal || actual == workerDesired
}

type vpcWorkerReadinessOptions struct {
	RequireNonEmpty bool
	FailOnUnhealthy bool
}

func workersHavePoolIdentity(workers []v2.Worker) bool {
	for _, worker := range workers {
		if worker.PoolID != "" || worker.PoolName != "" {
			return true
		}
	}
	return false
}

func evaluateVpcWorkerPoolReadiness(workers []v2.Worker, poolNameOrID string, opts vpcWorkerReadinessOptions) (string, error) {
	poolWorkers := filterWorkersByPool(workers, poolNameOrID)
	if poolNameOrID != "" && len(poolWorkers) == 0 && len(workers) > 0 && !workersHavePoolIdentity(workers) {
		poolWorkers = filterWorkersByPool(workers, "")
	}
	if opts.RequireNonEmpty && len(poolWorkers) == 0 {
		return workerProvisionPending, nil
	}

	for _, worker := range poolWorkers {
		if opts.FailOnUnhealthy && workerIsFailedOrCritical(worker) {
			return "", fmt.Errorf("[ERROR] Worker %s in pool %s is unhealthy: health=%q lifecycle=%q", worker.ID, poolNameOrID, worker.Health.State, worker.LifeCycle.ActualState)
		}
		if !workerIsReady(worker) {
			return workerProvisionPending, nil
		}
	}
	return workerDesired, nil
}

func workersForPool(workers []v2.Worker, poolID, poolName string) []v2.Worker {
	if poolID == "" && poolName == "" {
		return filterWorkersByPool(workers, "")
	}
	var matched []v2.Worker
	if poolID != "" {
		matched = filterWorkersByPool(workers, poolID)
	}
	if poolName != "" && poolName != poolID {
		matched = mergeWorkersByID(matched, filterWorkersByPool(workers, poolName))
	}
	return matched
}

func listVpcWorkerPoolWorkers(workersAPI v2.Workers, cluster, poolID, poolName string, target v2.ClusterTargetHeader) ([]v2.Worker, error) {
	poolRef := poolID
	if poolRef == "" {
		poolRef = poolName
	}
	if poolRef == "" {
		return []v2.Worker{}, nil
	}

	workers, err := workersAPI.ListByWorkerPool(cluster, poolRef, false, target)
	if err != nil {
		return nil, fmt.Errorf("[ERROR] Error retrieving workers for worker pool %s: %s", poolRef, err)
	}
	matched := workersForPool(workers, poolID, poolName)
	if len(matched) == 0 && len(workers) > 0 && !workersHavePoolIdentity(workers) {
		return filterWorkersByPool(workers, ""), nil
	}
	return matched, nil
}
