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

// Package output renders gocheck results as text or JSON.
package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/yeboyzq/gocheck/app/modules/report"
)

var tableHeaders = []string{"PACKAGE", "CURRENT", "LATEST", "NOTE"}

// Table renders an aligned table. Errors are always retained.
func Table(results []report.DependencyResult, includeAll bool) string {
	rows := visibleResults(results, includeAll)
	if len(rows) == 0 {
		return "All direct dependencies are up to date.\n"
	}

	table := make([][]string, 0, len(rows)+1)
	table = append(table, tableHeaders)
	for _, result := range rows {
		table = append(table, []string{
			result.Path,
			result.Current,
			result.Latest,
			note(result),
		})
	}

	widths := make([]int, len(tableHeaders))
	for _, row := range table {
		for column, cell := range row {
			if length := utf8.RuneCountInString(cell); length > widths[column] {
				widths[column] = length
			}
		}
	}

	var buffer bytes.Buffer
	for rowIndex, row := range table {
		if rowIndex > 0 {
			buffer.WriteByte('\n')
		}
		for column, cell := range row {
			if column > 0 {
				buffer.WriteString(" | ")
			}
			buffer.WriteString(cell)
			buffer.WriteString(strings.Repeat(" ", widths[column]-utf8.RuneCountInString(cell)))
		}
	}
	buffer.WriteByte('\n')
	return buffer.String()
}

// JSON renders the stable --json payload.
func JSON(results []report.DependencyResult, includeAll bool) (string, error) {
	payload := report.Payload{Dependencies: visibleResults(results, includeAll)}
	if payload.Dependencies == nil {
		payload.Dependencies = []report.DependencyResult{}
	}
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode JSON: %w", err)
	}
	return string(encoded) + "\n", nil
}

func visibleResults(results []report.DependencyResult, includeAll bool) []report.DependencyResult {
	visible := make([]report.DependencyResult, 0, len(results))
	for _, result := range results {
		if includeAll || result.UpdateAvailable || result.Error != "" {
			visible = append(visible, result)
		}
	}
	return visible
}

func note(result report.DependencyResult) string {
	parts := make([]string, 0, 3)
	if result.Major {
		parts = append(parts, "major")
	}
	if result.Replace {
		parts = append(parts, "replace")
	}
	if result.PreRelease {
		parts = append(parts, "pre-release")
	}
	if result.Error != "" {
		parts = append(parts, "error: "+singleLine(result.Error))
	}
	return strings.Join(parts, ", ")
}

func singleLine(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
