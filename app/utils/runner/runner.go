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

// Package runner abstracts external command execution for tests.
package runner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Runner executes a command and returns captured standard output and error.
type Runner interface {
	Run(ctx context.Context, dir string, env []string, name string, args ...string) (string, string, error)
}

// Exec executes commands with the real operating system.
type Exec struct{}

// Run captures stdout and stderr and enforces ctx's deadline.
func (Exec) Run(ctx context.Context, dir string, env []string, name string, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append([]string(nil), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if ctx.Err() != nil {
		return stdout.String(), stderr.String(), ctx.Err()
	}
	if err != nil {
		return stdout.String(), stderr.String(), fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return stdout.String(), stderr.String(), nil
}

// WithoutGOFLAGS returns env with GOFLAGS removed so gocheck can force readonly mode.
func WithoutGOFLAGS(env []string) []string {
	filtered := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if entry == "GOFLAGS" || strings.HasPrefix(entry, "GOFLAGS=") {
			continue
		}
		filtered = append(filtered, entry)
	}
	return append(filtered, "GOFLAGS=")
}

// WithGitPromptDisabled returns env with interactive Git authentication disabled.
func WithGitPromptDisabled(env []string) []string {
	return append(append([]string(nil), env...), "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=echo")
}

// Env exposes the current process environment.
func Env() []string { return os.Environ() }
