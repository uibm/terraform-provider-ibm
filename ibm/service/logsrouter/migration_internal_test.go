// Copyright IBM Corp. 2026 All Rights Reserved.
// Licensed under the Mozilla Public License v2.0

package logsrouter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/platform-services-go-sdk/logsrouterv3"
)

// fakeMigrationAPI is a small model of the /migrate state machine.
type fakeMigrationAPI struct {
	mu          sync.Mutex
	state       string
	statusCode  int // forced status for GET /migrate when non-zero
	inProgress  int // GETs that report IN_PROGRESS after generate
	calls       []string
	queryBodies []string
}

func (f *fakeMigrationAPI) handler(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	call := r.Method + " " + r.URL.Path
	if a := r.URL.Query().Get("action"); a != "" {
		call += "?action=" + a
	}
	f.calls = append(f.calls, call)
	write := func(code int) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		version := 1
		if f.state == migrationStateComplete {
			version = 3
		}
		_, _ = fmt.Fprintf(w, `{"api_version":%d,"state":%q,"message":"state is %s"}`, version, f.state, f.state)
	}
	switch call {
	case "GET /v3/migrate":
		if f.statusCode != 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(f.statusCode)
			_, _ = w.Write([]byte(`{"errors":[{"code":"not_found"}]}`))
			return
		}
		if f.state == migrationStateInProgress {
			if f.inProgress == 0 {
				f.state = migrationStatePendingCompletion
			} else {
				f.inProgress--
			}
		}
		write(http.StatusOK)
	case "POST /v3/migrate?action=generate":
		f.state = migrationStateInProgress
		write(http.StatusAccepted)
	case "POST /v3/migrate?action=complete":
		f.state = migrationStateComplete
		write(http.StatusOK)
	case "DELETE /v3/migrate":
		if f.state == migrationStateComplete || f.state == migrationStateInProgress {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			_, _ = w.Write([]byte(`{"errors":[{"code":"method_not_allowed"}]}`))
			return
		}
		f.state = migrationStateBefore
		write(http.StatusOK)
	case "POST /v3/destinations/query":
		b, _ := io.ReadAll(r.Body)
		f.queryBodies = append(f.queryBodies, string(b))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"destinations":{"crn:a":[{"account":"acct","action":"send","crn":"crn:logs","region":"eu-de","service_name":"logs",
			"associated_targets":[{"crn":"crn:t","id":"t1","name":"target","association_reasons":[{"type":"route","name":"r1","crn":"crn:r"}]}]}],"crn:b":[]}}`))
	default:
		http.NotFound(w, r)
	}
}

func newFakeClient(t *testing.T, f *fakeMigrationAPI) *logsrouterv3.LogsRouterV3 {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(f.handler))
	t.Cleanup(srv.Close)
	client, err := logsrouterv3.NewLogsRouterV3(&logsrouterv3.LogsRouterV3Options{URL: srv.URL + "/v3", Authenticator: &core.NoAuthAuthenticator{}})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestMigrationGenerateWaitComplete(t *testing.T) {
	f := &fakeMigrationAPI{state: migrationStateBefore, inProgress: 2}
	client := newFakeClient(t, f)
	ctx := context.Background()

	if _, _, err := client.MigrateActionsWithContext(ctx, client.NewMigrateActionsOptions(logsrouterv3.MigrateActionsOptionsActionGenerateConst)); err != nil {
		t.Fatal(err)
	}
	state, err := waitForMigrationState(ctx, client, time.Minute, time.Millisecond,
		[]string{migrationStateBefore, migrationStateInProgress}, []string{migrationStatePendingCompletion, migrationStateComplete})
	if err != nil || *state.State != migrationStatePendingCompletion {
		t.Fatalf("wait generate: %v %v", state, err)
	}
	// complete then reports COMPLETE on API version 3.
	if _, _, err := client.MigrateActionsWithContext(ctx, client.NewMigrateActionsOptions(logsrouterv3.MigrateActionsOptionsActionCompleteConst)); err != nil {
		t.Fatal(err)
	}
	state, err = getMigrationState(ctx, client)
	if err != nil || *state.State != migrationStateComplete || *state.APIVersion != 3 {
		t.Fatalf("after complete: %+v %v", state, err)
	}
	want := "POST /v3/migrate?action=generate,GET /v3/migrate,GET /v3/migrate,GET /v3/migrate,POST /v3/migrate?action=complete,GET /v3/migrate"
	if got := strings.Join(f.calls, ","); got != want {
		t.Errorf("calls:\n got %s\nwant %s", got, want)
	}
}

func TestMigrationWaitTimeoutIncludesMessage(t *testing.T) {
	f := &fakeMigrationAPI{state: migrationStateInProgress, inProgress: 1 << 30}
	client := newFakeClient(t, f)
	_, err := waitForMigrationState(context.Background(), client, 50*time.Millisecond, time.Millisecond,
		[]string{migrationStateInProgress}, []string{migrationStatePendingCompletion})
	if err == nil || !strings.Contains(err.Error(), "last state IN_PROGRESS: state is IN_PROGRESS") {
		t.Fatalf("want timeout with message, got %v", err)
	}
}

func TestMigrationNotFoundMeansComplete(t *testing.T) {
	client := newFakeClient(t, &fakeMigrationAPI{statusCode: http.StatusNotFound})
	state, err := getMigrationState(context.Background(), client)
	if err != nil || *state.State != migrationStateComplete || *state.APIVersion != 3 {
		t.Fatalf("got %+v %v", state, err)
	}
	client = newFakeClient(t, &fakeMigrationAPI{statusCode: http.StatusForbidden})
	if _, err := getMigrationState(context.Background(), client); err == nil {
		t.Fatal("403 must be an error")
	}
}

func TestMigrationReset(t *testing.T) {
	f := &fakeMigrationAPI{state: migrationStatePendingCompletion}
	if err := resetMigration(context.Background(), newFakeClient(t, f)); err != nil || f.state != migrationStateBefore {
		t.Fatalf("reset: %v state=%s", err, f.state)
	}
	f = &fakeMigrationAPI{state: migrationStateComplete}
	err := resetMigration(context.Background(), newFakeClient(t, f))
	if err == nil || !strings.Contains(err.Error(), "cannot be reset in its current state") {
		t.Fatalf("want clear 405 message, got %v", err)
	}
}

func TestDestinationsQueryFlatten(t *testing.T) {
	f := &fakeMigrationAPI{}
	client := newFakeClient(t, f)
	crns := []string{"crn:a", "crn:b", "crn:missing"}
	protos := []logsrouterv3.CRNPrototype{}
	for i := range crns {
		protos = append(protos, logsrouterv3.CRNPrototype{CRN: &crns[i]})
	}
	result, _, err := client.QueryDestinationsWithContext(context.Background(), client.NewQueryDestinationsOptions(protos))
	if err != nil {
		t.Fatal(err)
	}
	var body map[string][]map[string]string
	if err := json.Unmarshal([]byte(f.queryBodies[0]), &body); err != nil || body["crns"][1]["crn"] != "crn:b" {
		t.Fatalf("request body %s", f.queryBodies[0])
	}
	out := flattenDestinationsQuery(crns, result)
	if len(out) != 3 || out[0]["crn"] != "crn:a" || out[2]["crn"] != "crn:missing" || len(out[2]["items"].([]map[string]interface{})) != 0 {
		t.Fatalf("flatten = %v", out)
	}
	item := out[0]["items"].([]map[string]interface{})[0]
	target := item["associated_targets"].([]map[string]interface{})[0]
	reason := target["association_reasons"].([]map[string]interface{})[0]
	if item["action"] != "send" || item["region"] != "eu-de" || target["id"] != "t1" || reason["name"] != "r1" {
		t.Errorf("item = %v", item)
	}
}

func TestMigrationSchemas(t *testing.T) {
	if err := ResourceIBMLogsRouterMigration().InternalValidate(nil, true); err != nil {
		t.Fatal(err)
	}
	if err := DataSourceIBMLogsRouterMigration().InternalValidate(nil, false); err != nil {
		t.Fatal(err)
	}
	if err := DataSourceIBMLogsRouterDestinations().InternalValidate(nil, false); err != nil {
		t.Fatal(err)
	}
}
