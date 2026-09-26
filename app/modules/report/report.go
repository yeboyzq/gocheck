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

// Package report defines gocheck's stable result model.
package report

// DependencyResult is the outcome for one direct dependency.
type DependencyResult struct {
	Path            string `json:"path"`
	Current         string `json:"current"`
	Latest          string `json:"latest"`
	UpdateAvailable bool   `json:"update_available"`
	Major           bool   `json:"major"`
	Replace         bool   `json:"replace"`
	PreRelease      bool   `json:"pre_release,omitempty"`
	Error           string `json:"error"`
}

// Payload is the root object emitted by --json.
type Payload struct {
	Dependencies []DependencyResult `json:"dependencies"`
}
