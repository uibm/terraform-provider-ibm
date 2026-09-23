// Copyright IBM Corp. 2017, 2026 All Rights Reserved.
// Licensed under the Mozilla Public License v2.0

package kubernetes

import (
	"strings"
	"testing"

	v2 "github.com/IBM-Cloud/bluemix-go/api/container/containerv2"
	"github.com/stretchr/testify/assert"
)

func testVpcWorker(id, poolID, poolName, health, actual string) v2.Worker {
	return v2.Worker{
		ID:       id,
		PoolID:   poolID,
		PoolName: poolName,
		Flavor:   "bx2.4x16",
		Location: "us-south-1",
		Health:   v2.HealthStatus{State: health},
		LifeCycle: v2.WorkerLifeCycle{
			ActualState: actual,
		},
		KubeVersion: v2.KubeDetails{Actual: "1.32.0"},
	}
}

func TestFilterWorkersByPool(t *testing.T) {
	workers := []v2.Worker{
		testVpcWorker("w1", "pool-1", "default", "normal", "deployed"),
		testVpcWorker("w2", "pool-2", "extra", "normal", "deployed"),
		testVpcWorker("w1", "pool-1", "default", "normal", "deployed"),
		testVpcWorker("w3", "pool-1", "default", "", "provisioning"),
	}

	t.Run("nil", func(t *testing.T) {
		assert.Empty(t, filterWorkersByPool(nil, "default"))
	})

	t.Run("empty", func(t *testing.T) {
		assert.Empty(t, filterWorkersByPool([]v2.Worker{}, "default"))
	})

	t.Run("mixed pools and duplicate ids", func(t *testing.T) {
		got := filterWorkersByPool(workers, "default")
		assert.Equal(t, []string{"w1", "w3"}, []string{got[0].ID, got[1].ID})
	})

	t.Run("match by pool id", func(t *testing.T) {
		got := filterWorkersByPool(workers, "pool-2")
		assert.Len(t, got, 1)
		assert.Equal(t, "w2", got[0].ID)
	})

	t.Run("no pool filter unique ids", func(t *testing.T) {
		got := filterWorkersByPool(workers, "")
		assert.Equal(t, []string{"w1", "w2", "w3"}, []string{got[0].ID, got[1].ID, got[2].ID})
	})
}

func TestWorkersForPool(t *testing.T) {
	workers := []v2.Worker{
		testVpcWorker("w1", "pool-1", "default", "normal", "deployed"),
		testVpcWorker("w2", "pool-2", "extra", "failed", "deploy_failed"),
	}

	t.Run("name only does not include other pools", func(t *testing.T) {
		got := workersForPool(workers, "", "default")
		assert.Len(t, got, 1)
		assert.Equal(t, "w1", got[0].ID)
	})

	t.Run("id and name merge unique workers", func(t *testing.T) {
		got := workersForPool(workers, "pool-1", "default")
		assert.Len(t, got, 1)
		assert.Equal(t, "w1", got[0].ID)
	})
}

func TestFlattenVpcWorkerPoolWorkers(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		assert.Empty(t, flattenVpcWorkerPoolWorkers(nil))
	})

	t.Run("empty", func(t *testing.T) {
		assert.Empty(t, flattenVpcWorkerPoolWorkers([]v2.Worker{}))
	})

	t.Run("zero health values", func(t *testing.T) {
		got := flattenVpcWorkerPoolWorkers([]v2.Worker{
			testVpcWorker("w1", "pool-1", "default", "", ""),
		})
		assert.Equal(t, "w1", got[0]["id"])
		assert.Equal(t, "", got[0]["state"])
		assert.Equal(t, "", got[0]["lifecycle_actual_state"])
		assert.Equal(t, "bx2.4x16", got[0]["flavor"])
		assert.Equal(t, "1.32.0", got[0]["kube_version"])
		assert.Equal(t, "us-south-1", got[0]["location"])
	})
}

func TestNestVpcWorkerPoolWorkers(t *testing.T) {
	t.Run("nil pools", func(t *testing.T) {
		assert.Empty(t, nestVpcWorkerPoolWorkers(nil, []v2.Worker{testVpcWorker("w1", "p1", "default", "normal", "deployed")}))
	})

	t.Run("mixed pools", func(t *testing.T) {
		flattened := []map[string]interface{}{
			{"id": "p1", "name": "default"},
			{"id": "p2", "name": "extra"},
		}
		workers := []v2.Worker{
			testVpcWorker("w1", "p1", "default", "normal", "deployed"),
			testVpcWorker("w2", "p2", "extra", "warning", "provisioning"),
			testVpcWorker("w1", "p1", "default", "normal", "deployed"),
		}

		got := nestVpcWorkerPoolWorkers(flattened, workers)
		defaultWorkers := got[0]["workers"].([]map[string]interface{})
		extraWorkers := got[1]["workers"].([]map[string]interface{})
		assert.Len(t, defaultWorkers, 1)
		assert.Equal(t, "w1", defaultWorkers[0]["id"])
		assert.Equal(t, "normal", defaultWorkers[0]["state"])
		assert.Len(t, extraWorkers, 1)
		assert.Equal(t, "w2", extraWorkers[0]["id"])
	})
}

func TestEvaluateVpcWorkerPoolReadiness(t *testing.T) {
	opts := vpcWorkerReadinessOptions{RequireNonEmpty: true, FailOnUnhealthy: true}

	t.Run("nil and empty stay pending", func(t *testing.T) {
		state, err := evaluateVpcWorkerPoolReadiness(nil, "default", opts)
		assert.NoError(t, err)
		assert.Equal(t, workerProvisionPending, state)

		state, err = evaluateVpcWorkerPoolReadiness([]v2.Worker{}, "default", opts)
		assert.NoError(t, err)
		assert.Equal(t, workerProvisionPending, state)
	})

	t.Run("pending when workers are still provisioning", func(t *testing.T) {
		state, err := evaluateVpcWorkerPoolReadiness([]v2.Worker{
			testVpcWorker("w1", "p1", "default", "", "provisioning"),
		}, "default", opts)
		assert.NoError(t, err)
		assert.Equal(t, workerProvisionPending, state)
	})

	t.Run("ready when health is normal", func(t *testing.T) {
		state, err := evaluateVpcWorkerPoolReadiness([]v2.Worker{
			testVpcWorker("w1", "p1", "default", "normal", "provisioning"),
		}, "default", opts)
		assert.NoError(t, err)
		assert.Equal(t, workerDesired, state)
	})

	t.Run("ready when lifecycle is deployed", func(t *testing.T) {
		state, err := evaluateVpcWorkerPoolReadiness([]v2.Worker{
			testVpcWorker("w1", "p1", "default", "", "deployed"),
		}, "default", opts)
		assert.NoError(t, err)
		assert.Equal(t, workerDesired, state)
	})

	t.Run("failed health is an error", func(t *testing.T) {
		_, err := evaluateVpcWorkerPoolReadiness([]v2.Worker{
			testVpcWorker("w1", "p1", "default", "failed", "provisioning"),
		}, "default", opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "w1")
	})

	t.Run("critical health is an error", func(t *testing.T) {
		_, err := evaluateVpcWorkerPoolReadiness([]v2.Worker{
			testVpcWorker("w1", "p1", "default", "critical", "deployed"),
		}, "default", opts)
		assert.Error(t, err)
	})

	t.Run("deploy_failed lifecycle is an error", func(t *testing.T) {
		_, err := evaluateVpcWorkerPoolReadiness([]v2.Worker{
			testVpcWorker("w1", "p1", "default", "", "deploy_failed"),
		}, "default", opts)
		assert.Error(t, err)
	})

	t.Run("extra pool workers are ignored", func(t *testing.T) {
		state, err := evaluateVpcWorkerPoolReadiness([]v2.Worker{
			testVpcWorker("w1", "p1", "default", "normal", "deployed"),
			testVpcWorker("w2", "p2", "extra", "failed", "deploy_failed"),
		}, "default", opts)
		assert.NoError(t, err)
		assert.Equal(t, workerDesired, state)
	})

	t.Run("duplicate ids are evaluated once", func(t *testing.T) {
		state, err := evaluateVpcWorkerPoolReadiness([]v2.Worker{
			testVpcWorker("w1", "p1", "default", "normal", "deployed"),
			testVpcWorker("w1", "p1", "default", "normal", "deployed"),
		}, "default", opts)
		assert.NoError(t, err)
		assert.Equal(t, workerDesired, state)
	})

	t.Run("workers without pool identity still count", func(t *testing.T) {
		state, err := evaluateVpcWorkerPoolReadiness([]v2.Worker{
			testVpcWorker("w1", "", "", "normal", "deployed"),
		}, "default", opts)
		assert.NoError(t, err)
		assert.Equal(t, workerDesired, state)
	})

	t.Run("mixed pending and ready stays pending", func(t *testing.T) {
		state, err := evaluateVpcWorkerPoolReadiness([]v2.Worker{
			testVpcWorker("w1", "p1", "default", "normal", "deployed"),
			testVpcWorker("w2", "p1", "default", "", "provisioning"),
		}, "default", opts)
		assert.NoError(t, err)
		assert.Equal(t, workerProvisionPending, state)
	})
}

func TestVpcClusterWaitTillValues(t *testing.T) {
	values := vpcClusterWaitTillValues()
	assert.Contains(t, values, allWorkersReady)
	assert.Contains(t, values, masterNodeReady)
	assert.Contains(t, values, oneWorkerNodeReady)
	assert.Contains(t, values, ingressReady)
	assert.Contains(t, values, clusterNormal)
	assert.Equal(t, 5, len(values))
	assert.True(t, strings.EqualFold(allWorkersReady, "allworkersready"))
}

func TestVpcWorkerPoolWorkersSchema(t *testing.T) {
	schema := vpcWorkerPoolWorkersSchema()
	assert.True(t, schema.Computed)
	elem := schema.Elem
	assert.NotNil(t, elem)
}
