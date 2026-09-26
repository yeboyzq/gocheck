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

// Package version provides the semantic-version rules used by gocheck.
package version

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

// IsValid reports whether value is a valid Go semantic version.
func IsValid(value string) bool {
	return semver.IsValid(value)
}

// IsRelease reports whether value is a valid version without a pre-release.
func IsRelease(value string) bool {
	return semver.IsValid(value) && semver.Prerelease(value) == ""
}

// Compare compares two valid semantic versions. Build metadata is ignored.
func Compare(left, right string) int {
	return semver.Compare(left, right)
}

// Major returns the numeric major version encoded by a module path.
func Major(path string) int {
	_, pathMajor, ok := module.SplitPathVersion(path)
	if !ok || pathMajor == "" {
		return 1
	}
	return majorFromPath(pathMajor)
}

// VersionMajor returns the numeric major version of a valid semantic version.
func VersionMajor(value string) int {
	if !semver.IsValid(value) {
		return 1
	}
	return majorFromPath(semver.Major(value))
}

// CandidatePaths returns module paths for majors strictly after path and current.
// The maximum supported major is ten, as agreed for v1.
func CandidatePaths(path, current string, maximum int) []string {
	prefix, _, ok := module.SplitPathVersion(path)
	if !ok || prefix == "" {
		return nil
	}

	start := Major(path)
	if IsValid(current) {
		if currentMajor := VersionMajor(current); currentMajor > start {
			start = currentMajor
		}
	}
	if start >= maximum {
		return nil
	}

	candidates := make([]string, 0, maximum-start)
	for major := start + 1; major <= maximum; major++ {
		candidates = append(candidates, fmt.Sprintf("%s/v%d", prefix, major))
	}
	return candidates
}

func majorFromPath(value string) int {
	value = strings.TrimPrefix(value, "/")
	value = strings.TrimPrefix(value, "v")
	major, err := strconv.Atoi(value)
	if err != nil || major < 0 {
		return 1
	}
	return major
}
