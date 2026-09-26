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

package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func executeForTest(command *cobra.Command, args ...string) (string, string, error) {
	var stdout, stderr bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SetArgs(args)
	err := command.Execute()
	return stdout.String(), stderr.String(), err
}

func TestConcurrencyMustBePositive(t *testing.T) {
	_, _, err := executeForTest(newRootCommand(), "--concurrency", "0", "--dir", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "positive integer") {
		t.Fatalf("Execute() error = %v, want positive integer error", err)
	}
}

func TestTargetMustContainGoMod(t *testing.T) {
	_, _, err := executeForTest(newRootCommand(), "--dir", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "go.mod") {
		t.Fatalf("Execute() error = %v, want go.mod error", err)
	}
}

func TestNoDependenciesSucceeds(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "go.mod")
	if err := os.WriteFile(path, []byte("module example.com/empty\n\ngo 1.23\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, _, err := executeForTest(newRootCommand(), "--dir", directory)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if stdout != "All direct dependencies are up to date.\n" {
		t.Fatalf("stdout = %q", stdout)
	}
}

func TestRootCommandHasNoBusinessSubcommands(t *testing.T) {
	command := newRootCommand()
	if command.CompletionOptions.DisableDefaultCmd != true {
		t.Fatal("default completion command should be disabled")
	}
	for _, child := range command.Commands() {
		if child.Name() == "completion" {
			t.Fatal("completion command should not be registered")
		}
	}
}
