// Copyright IBM Corp. 2025 All Rights Reserved.
// Licensed under the Mozilla Public License v2.0

package vpc

import (
	"reflect"
	"testing"
)

func bmsAttachmentMaps(names ...string) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(names))
	for _, n := range names {
		out = append(out, map[string]interface{}{"name": n})
	}
	return out
}

func bmsAttachmentPrior(names ...string) []interface{} {
	out := make([]interface{}, 0, len(names))
	for _, n := range names {
		out = append(out, map[string]interface{}{"name": n})
	}
	return out
}

func bmsAttachmentNames(items []map[string]interface{}) []string {
	out := make([]string, 0, len(items))
	for _, m := range items {
		out = append(out, m["name"].(string))
	}
	return out
}

func TestOrderBareMetalServerNetworkAttachments(t *testing.T) {
	cases := []struct {
		name    string
		fetched []string
		prior   []interface{}
		want    []string
	}{
		{
			name:    "api order differs from state order",
			fetched: []string{"pci5", "pci8", "pci2", "pci7", "pci3", "pci6", "pci4"},
			prior:   bmsAttachmentPrior("pci2", "pci3", "pci4", "pci5", "pci6", "pci7", "pci8"),
			want:    []string{"pci2", "pci3", "pci4", "pci5", "pci6", "pci7", "pci8"},
		},
		{
			name:    "state order is kept even when it is not alphabetical",
			fetched: []string{"a-pci", "c-pci", "b-pci"},
			prior:   bmsAttachmentPrior("c-pci", "a-pci", "b-pci"),
			want:    []string{"c-pci", "a-pci", "b-pci"},
		},
		{
			name:    "attachments unknown to state follow, sorted by name",
			fetched: []string{"vlan-b", "pci3", "vlan-a", "pci2"},
			prior:   bmsAttachmentPrior("pci2", "pci3"),
			want:    []string{"pci2", "pci3", "vlan-a", "vlan-b"},
		},
		{
			name:    "no prior state sorts by name",
			fetched: []string{"pci3", "pci1", "pci2"},
			prior:   nil,
			want:    []string{"pci1", "pci2", "pci3"},
		},
		{
			name:    "attachment removed outside terraform keeps the rest in order",
			fetched: []string{"pci4", "pci2"},
			prior:   bmsAttachmentPrior("pci2", "pci3", "pci4"),
			want:    []string{"pci2", "pci4"},
		},
		{
			name:    "empty input",
			fetched: nil,
			prior:   bmsAttachmentPrior("pci2"),
			want:    []string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := bmsAttachmentNames(orderBareMetalServerNetworkAttachments(bmsAttachmentMaps(tc.fetched...), tc.prior))
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
