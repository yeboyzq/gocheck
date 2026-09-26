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

// Package modfile reads direct requirements from a target project's go.mod.
package modfile

import (
	"fmt"
	"os"

	"golang.org/x/mod/modfile"
)

// Replacement describes the effective replacement for a direct requirement.
type Replacement struct {
	Path    string
	Version string
	Local   bool
}

// Dependency is one direct requirement from go.mod.
type Dependency struct {
	Path        string
	Current     string
	Replacement *Replacement
}

// File contains the direct dependency information gocheck needs.
type File struct {
	ModulePath   string
	Dependencies []Dependency
}

// Parse reads path and returns only non-indirect requirements.
func Parse(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read go.mod: %w", err)
	}

	// ParseLax intentionally ignores main-module directives such as replace.
	// A target go.mod is always a main module, so strict parsing is required here.
	parsed, err := modfile.Parse(path, data, nil)
	if err != nil {
		return nil, fmt.Errorf("parse go.mod: %w", err)
	}

	replacements := make(map[string]*modfile.Replace)
	for _, replace := range parsed.Replace {
		if replace == nil {
			continue
		}
		replacements[replace.Old.Path] = replace
	}

	file := &File{ModulePath: parsed.Module.Mod.Path}
	seen := make(map[string]struct{}, len(parsed.Require))
	for _, require := range parsed.Require {
		if require == nil || require.Indirect {
			continue
		}
		if _, exists := seen[require.Mod.Path]; exists {
			return nil, fmt.Errorf("duplicate direct requirement %q", require.Mod.Path)
		}
		seen[require.Mod.Path] = struct{}{}

		dependency := Dependency{
			Path:    require.Mod.Path,
			Current: require.Mod.Version,
		}
		if replace := replacementFor(replacements, dependency.Path, dependency.Current); replace != nil {
			dependency.Replacement = &Replacement{
				Path:    replace.New.Path,
				Version: replace.New.Version,
				Local:   replace.New.Version == "",
			}
		}
		file.Dependencies = append(file.Dependencies, dependency)
	}

	return file, nil
}

func replacementFor(replacements map[string]*modfile.Replace, path, version string) *modfile.Replace {
	replace := replacements[path]
	if replace == nil {
		return nil
	}
	if replace.Old.Version == "" || replace.Old.Version == version {
		return replace
	}
	return nil
}
