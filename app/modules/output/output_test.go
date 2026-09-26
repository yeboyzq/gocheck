/*
Copyright (c) 2026 gocheck
gocheck is licensed under Mulan PSL v2.
You can use this software according to the terms and conditions of the Mulan PSL v2.
You may obtain a copy of Mulan PSL v2 at:
        http://license.coscl.org.cn/MulanPSL2
THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND,
EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT,
MERCHANTABILITY OR FIT FOR A PARTICULAR PURPOSE.
See the Mulan PSL v2 for more details.
*/

package output

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yeboyzq/gocheck/app/modules/report"
)

func sampleResults() []report.DependencyResult {
	return []report.DependencyResult{
		{Path: "example.com/current", Current: "v1.0.0", Latest: "v1.0.0"},
		{Path: "example.com/update", Current: "v1.0.0", Latest: "v1.2.0", UpdateAvailable: true},
		{Path: "example.com/major", Current: "v1.0.0", Latest: "v2.0.0", UpdateAvailable: true, Major: true},
		{Path: "example.com/local", Current: "local", Replace: true},
		{Path: "example.com/error", Current: "v1.0.0", Error: "query failed"},
	}
}

func TestTableDefaultShowsUpdatesAndErrors(t *testing.T) {
	table := Table(sampleResults(), false)
	if strings.Contains(table, "example.com/current") {
		t.Fatalf("table unexpectedly contains current dependency:\n%s", table)
	}
	for _, path := range []string{"example.com/current", "example.com/local"} {
		if strings.Contains(table, path) {
			t.Fatalf("table unexpectedly contains %s:\n%s", path, table)
		}
	}
	if !strings.HasPrefix(table, "PACKAGE ") || !strings.Contains(table, "CURRENT | LATEST | NOTE") {
		t.Fatalf("table missing header:\n%s", table)
	}
	if !strings.Contains(table, "major") || !strings.Contains(table, "error: query failed") {
		t.Fatalf("table missing notes:\n%s", table)
	}
}

func TestTableAllShowsEveryResult(t *testing.T) {
	table := Table(sampleResults(), true)
	for _, path := range []string{
		"example.com/current", "example.com/update", "example.com/major", "example.com/local", "example.com/error",
	} {
		if !strings.Contains(table, path) {
			t.Fatalf("table missing %s:\n%s", path, table)
		}
	}
}

func TestJSONSchemaAndFiltering(t *testing.T) {
	encoded, err := JSON(sampleResults(), false)
	if err != nil {
		t.Fatalf("JSON() error = %v", err)
	}
	var payload struct {
		Dependencies []map[string]any `json:"dependencies"`
	}
	if err := json.Unmarshal([]byte(encoded), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, encoded)
	}
	if len(payload.Dependencies) != 3 {
		t.Fatalf("dependencies length = %d, want 3", len(payload.Dependencies))
	}
	wantKeys := []string{"path", "current", "latest", "update_available", "major", "replace", "error"}
	for index, dependency := range payload.Dependencies {
		for _, key := range wantKeys {
			if _, exists := dependency[key]; !exists {
				t.Fatalf("dependency %d missing key %q: %v", index, key, dependency)
			}
		}
	}
}

func TestEmptyOutput(t *testing.T) {
	if table := Table(nil, false); table != "All direct dependencies are up to date.\n" {
		t.Fatalf("Table(nil) = %q", table)
	}
	encoded, err := JSON(nil, false)
	if err != nil {
		t.Fatalf("JSON() error = %v", err)
	}
	if !strings.Contains(encoded, `"dependencies": []`) {
		t.Fatalf("JSON(nil) should contain an empty array:\n%s", encoded)
	}
}
