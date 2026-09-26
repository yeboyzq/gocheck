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

// Package cmd implements the gocheck command line.
package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/yeboyzq/gocheck/app/modules/modfile"
	"github.com/yeboyzq/gocheck/app/modules/output"
	"github.com/yeboyzq/gocheck/app/modules/resolver"
)

type options struct {
	dir         string
	all         bool
	json        bool
	concurrency int
}

func newRootCommand() *cobra.Command {
	var opts options

	command := &cobra.Command{
		Use:          "gocheck",
		Short:        "Check direct Go module dependencies for available updates",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(command *cobra.Command, args []string) error {
			return run(command, opts)
		},
	}
	command.CompletionOptions.DisableDefaultCmd = true

	flags := command.Flags()
	flags.StringVar(&opts.dir, "dir", ".", "target Go project directory")
	flags.BoolVar(&opts.all, "all", false, "show dependencies without updates")
	flags.BoolVar(&opts.json, "json", false, "output JSON instead of a table")
	flags.IntVar(&opts.concurrency, "concurrency", 8, "number of concurrent dependency checks")

	command.PreRunE = func(command *cobra.Command, args []string) error {
		if opts.concurrency < 1 {
			return fmt.Errorf("--concurrency must be a positive integer")
		}
		absolute, err := filepath.Abs(opts.dir)
		if err != nil {
			return fmt.Errorf("resolve --dir: %w", err)
		}
		opts.dir = absolute
		return nil
	}

	return command
}

// Execute runs gocheck and maps execution failures to exit code one.
func Execute() {
	if err := newRootCommand().Execute(); err != nil {
		os.Exit(1)
	}
}

func run(command *cobra.Command, opts options) error {
	file, err := modfile.Parse(filepath.Join(opts.dir, "go.mod"))
	if err != nil {
		return err
	}

	dependencyResolver, cleanup, err := resolver.New(opts.concurrency)
	if err != nil {
		return err
	}
	defer cleanup()

	results := dependencyResolver.Resolve(context.Background(), file.Dependencies)
	if opts.json {
		encoded, err := output.JSON(results, opts.all)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprint(command.OutOrStdout(), encoded)
	} else {
		_, _ = fmt.Fprint(command.OutOrStdout(), output.Table(results, opts.all))
	}

	for _, result := range results {
		if result.Error != "" {
			return fmt.Errorf("dependency %s could not be checked", result.Path)
		}
	}
	return nil
}
