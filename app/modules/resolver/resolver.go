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

// Package resolver queries the latest available version of direct dependencies.
package resolver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/yeboyzq/gocheck/app/modules/gitquery"
	"github.com/yeboyzq/gocheck/app/modules/modfile"
	"github.com/yeboyzq/gocheck/app/modules/report"
	"github.com/yeboyzq/gocheck/app/modules/version"
	"github.com/yeboyzq/gocheck/app/utils/runner"
)

const (
	queryTimeout = 10 * time.Second
	maxMajor     = 10
)

var errNoVersion = errors.New("go list returned no version")

// GoList queries module versions through the go command.
type GoList struct {
	Runner     runner.Runner
	ScratchDir string
	Env        []string
}

// Latest resolves the latest version of modulePath. If upstream has no stable
// release, Go may return its newest pre-release version.
func (query GoList) Latest(ctx context.Context, modulePath string) (string, error) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	stdout, stderr, err := query.Runner.Run(
		queryCtx,
		query.ScratchDir,
		runner.WithoutGOFLAGS(query.Env),
		"go", "list", "-m", "-mod=readonly", "-json", modulePath+"@latest",
	)
	if err != nil {
		return "", fmt.Errorf("go list failed: %v: %s", err, firstLine(stderr))
	}

	var response struct {
		Path    string
		Version string
	}
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		return "", fmt.Errorf("decode go list output: %w", err)
	}
	if response.Version == "" {
		return "", errNoVersion
	}
	if !version.IsValid(response.Version) {
		return "", fmt.Errorf("go list returned invalid version %q", response.Version)
	}
	return response.Version, nil
}

// Resolver coordinates concurrent checks.
type Resolver struct {
	GoList      *GoList
	Git         *gitquery.Client
	Concurrency int
}

// Resolve checks all dependencies while preserving their input order.
func (resolver Resolver) Resolve(ctx context.Context, dependencies []modfile.Dependency) []report.DependencyResult {
	results := make([]report.DependencyResult, len(dependencies))
	if len(dependencies) == 0 {
		return results
	}

	work := make(chan int)
	var workers sync.WaitGroup
	limit := resolver.Concurrency
	if limit < 1 {
		limit = 1
	}
	for worker := 0; worker < limit; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for index := range work {
				results[index] = resolver.resolveOne(ctx, dependencies[index])
			}
		}()
	}
	for index := range dependencies {
		select {
		case work <- index:
		case <-ctx.Done():
		}
	}
	close(work)
	workers.Wait()
	return results
}

func (resolver Resolver) resolveOne(ctx context.Context, dependency modfile.Dependency) report.DependencyResult {
	result := report.DependencyResult{
		Path:    dependency.Path,
		Current: dependency.Current,
		Replace: dependency.Replacement != nil,
	}
	if dependency.Replacement != nil && dependency.Replacement.Local {
		result.Current = "local"
		return result
	}

	queryPath := dependency.Path
	effectiveCurrent := dependency.Current
	if dependency.Replacement != nil {
		queryPath = dependency.Replacement.Path
		effectiveCurrent = dependency.Replacement.Version
	}
	if !version.IsValid(effectiveCurrent) {
		result.Error = "current requirement is not a valid semantic version"
		return result
	}

	latest, err := resolver.latestForModule(ctx, queryPath)
	if err != nil {
		result.Error = "latest version unavailable: " + err.Error()
		return result
	}
	result.Latest = latest
	result.UpdateAvailable = version.Compare(effectiveCurrent, latest) < 0
	result.Major = isMajorModuleUpdate(effectiveCurrent, latest)
	result.PreRelease = !version.IsRelease(latest) || !version.IsRelease(effectiveCurrent)

	candidates := version.CandidatePaths(queryPath, effectiveCurrent, maxMajor)
	candidateVersions := make([]string, len(candidates))
	var candidateGroup sync.WaitGroup
	for index, candidate := range candidates {
		candidateGroup.Add(1)
		go func(index int, candidate string) {
			defer candidateGroup.Done()
			candidateLatest, candidateErr := resolver.latestForModule(ctx, candidate)
			if candidateErr == nil {
				candidateVersions[index] = candidateLatest
			}
		}(index, candidate)
	}
	candidateGroup.Wait()
	for _, candidateLatest := range candidateVersions {
		// Major module paths are only suggested when they have a stable release.
		// A pre-release-only /vN path is not a safe breaking-change recommendation.
		if version.IsRelease(candidateLatest) && version.Compare(candidateLatest, latest) > 0 {
			latest = candidateLatest
			result.Latest = latest
			result.UpdateAvailable = true
			result.Major = isMajorModuleUpdate(effectiveCurrent, latest)
			result.PreRelease = !version.IsRelease(effectiveCurrent)
		}
	}
	return result
}

func isMajorModuleUpdate(current, latest string) bool {
	currentMajor := version.VersionMajor(current)
	latestMajor := version.VersionMajor(latest)
	return latestMajor > currentMajor && latestMajor >= 2
}

func (resolver Resolver) latestForModule(ctx context.Context, modulePath string) (string, error) {
	latest, err := resolver.GoList.Latest(ctx, modulePath)
	if err == nil {
		return latest, nil
	}
	gitLatest, gitErr := resolver.Git.Latest(ctx, modulePath)
	if gitErr == nil {
		return gitLatest, nil
	}
	return "", fmt.Errorf("go list unavailable (%v); Git fallback unavailable (%v)", err, gitErr)
}

// New creates a production resolver and its temporary scratch module.
func New(concurrency int) (*Resolver, func(), error) {
	scratchDir, err := os.MkdirTemp("", "gocheck-scratch-")
	if err != nil {
		return nil, nil, fmt.Errorf("create scratch module: %w", err)
	}
	scratchMod := filepath.Join(scratchDir, "go.mod")
	content := "module gocheck.invalid/scratch\n\ngo 1.23\n"
	if err := os.WriteFile(scratchMod, []byte(content), 0o600); err != nil {
		_ = os.RemoveAll(scratchDir)
		return nil, nil, fmt.Errorf("initialize scratch module: %w", err)
	}

	env := runner.Env()
	cleanup := func() { _ = os.RemoveAll(scratchDir) }
	return &Resolver{
		Concurrency: concurrency,
		GoList:      &GoList{Runner: runner.Exec{}, ScratchDir: scratchDir, Env: env},
		Git:         gitquery.NewClient(env),
	}, cleanup, nil
}

func firstLine(value string) string {
	for index, char := range value {
		if char == '\n' || char == '\r' {
			return value[:index]
		}
	}
	return value
}
