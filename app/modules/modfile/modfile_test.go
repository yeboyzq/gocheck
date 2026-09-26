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

package modfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseDirectDependenciesAndReplacements(t *testing.T) {
	directory := t.TempDir()
	content := `module example.com/target

go 1.23

require (
	example.com/direct v1.2.3
	example.com/indirect/v2 v2.0.0 // indirect
	example.com/versioned/v3 v3.1.0
)

require example.com/single v0.1.0

replace example.com/direct => example.com/replacement v1.4.0

replace example.com/versioned/v3 => ../local
`
	path := filepath.Join(directory, "go.mod")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	file, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if file.ModulePath != "example.com/target" {
		t.Fatalf("ModulePath() = %q", file.ModulePath)
	}
	if len(file.Dependencies) != 3 {
		t.Fatalf("Dependencies length = %d, want 3", len(file.Dependencies))
	}

	first := file.Dependencies[0]
	if first.Path != "example.com/direct" || first.Current != "v1.2.3" {
		t.Fatalf("first dependency = %+v", first)
	}
	if first.Replacement == nil || first.Replacement.Path != "example.com/replacement" || first.Replacement.Version != "v1.4.0" || first.Replacement.Local {
		t.Fatalf("first replacement = %+v", first.Replacement)
	}

	second := file.Dependencies[1]
	if second.Path != "example.com/versioned/v3" || second.Replacement == nil || !second.Replacement.Local {
		t.Fatalf("second dependency = %+v", second)
	}

	third := file.Dependencies[2]
	if third.Path != "example.com/single" || third.Current != "v0.1.0" || third.Replacement != nil {
		t.Fatalf("third dependency = %+v", third)
	}
}

func TestParseRejectsDuplicateDirectRequirements(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "go.mod")
	content := "module example.com/target\n\ngo 1.23\n\nrequire example.com/direct v1.0.0\n\nrequire example.com/direct v1.1.0\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Parse(path); err == nil {
		t.Fatal("Parse() error = nil, want duplicate requirement error")
	}
}
