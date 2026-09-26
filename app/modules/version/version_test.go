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

package version

import "testing"

func TestReleaseAndComparisonRules(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		release bool
		major   int
	}{
		{name: "release", value: "v1.2.3", release: true, major: 1},
		{name: "incompatible release", value: "v2.0.0+incompatible", release: true, major: 2},
		{name: "prerelease", value: "v1.0.0-rc.1", release: false, major: 1},
		{name: "pseudo", value: "v0.0.0-20200101000000-abcdefabcdef", release: false, major: 0},
		{name: "invalid", value: "1.2.3", release: false, major: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsRelease(test.value); got != test.release {
				t.Fatalf("IsRelease(%q) = %v, want %v", test.value, got, test.release)
			}
			if got := VersionMajor(test.value); got != test.major {
				t.Fatalf("VersionMajor(%q) = %d, want %d", test.value, got, test.major)
			}
		})
	}

	if Compare("v1.2.3", "v1.10.0") >= 0 {
		t.Fatal("v1.2.3 should compare lower than v1.10.0")
	}
	if Compare("v2.0.0", "v2.0.0+incompatible") != 0 {
		t.Fatal("build metadata should not affect comparison")
	}
}

func TestCandidatePaths(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		current string
		first   string
		count   int
	}{
		{name: "v1", path: "example.com/module", current: "v1.2.3", first: "example.com/module/v2", count: 9},
		{name: "v3", path: "example.com/module/v3", current: "v3.2.0", first: "example.com/module/v4", count: 7},
		{name: "incompatible current", path: "example.com/module", current: "v2.0.0+incompatible", first: "example.com/module/v3", count: 8},
		{name: "maximum", path: "example.com/module/v10", current: "v10.0.0", count: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := CandidatePaths(test.path, test.current, 10)
			if len(got) != test.count {
				t.Fatalf("CandidatePaths() length = %d, want %d: %v", len(got), test.count, got)
			}
			if test.count > 0 && got[0] != test.first {
				t.Fatalf("CandidatePaths()[0] = %q, want %q", got[0], test.first)
			}
		})
	}
}
