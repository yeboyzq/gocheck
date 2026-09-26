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

// Package gitquery provides gocheck's Git-only private-module fallback.
package gitquery

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/mod/module"

	"github.com/yeboyzq/gocheck/app/modules/version"
	"github.com/yeboyzq/gocheck/app/utils/runner"
)

var (
	metaTagPattern = regexp.MustCompile(`(?i)<meta\b[^>]*>`)
	namePattern    = regexp.MustCompile(`(?i)\bname\s*=\s*(?:"go-import"|'go-import'|go-import)`)
	contentPattern = regexp.MustCompile(`(?i)\bcontent\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))`)
	downloadLimit  = int64(1024 * 1024)
	queryTimeout   = 10 * time.Second
)

// ErrUnsupportedVCS reports a go-import record for a VCS other than Git.
var ErrUnsupportedVCS = errors.New("VCS is not Git; only Git is supported")

// Client resolves stable Git tags and caches discovery results.
type Client struct {
	HTTPClient *http.Client
	Runner     runner.Runner
	Env        []string

	discoveryMu sync.Mutex
	discovery   map[string][]string
	unsupported map[string]struct{}
	tagsMu      sync.Mutex
	tags        map[string]map[int]string
}

// NewClient creates a production Git fallback client.
func NewClient(env []string) *Client {
	return &Client{
		HTTPClient:  &http.Client{},
		Runner:      runner.Exec{},
		Env:         env,
		discovery:   make(map[string][]string),
		unsupported: make(map[string]struct{}),
		tags:        make(map[string]map[int]string),
	}
}

// Latest returns the newest stable tag whose major matches modulePath.
func (client *Client) Latest(ctx context.Context, modulePath string) (string, error) {
	base, pathMajor, ok := module.SplitPathVersion(modulePath)
	if !ok {
		return "", fmt.Errorf("invalid module path")
	}
	desiredMajor := 1
	if pathMajor != "" {
		if value := majorFromPathMajor(pathMajor); value > 1 {
			desiredMajor = value
		}
	}

	repositories, err := client.repositories(ctx, base)
	if err != nil {
		return "", err
	}
	for _, repository := range repositories {
		tags, accessible := client.tagsFor(ctx, repository)
		if !accessible {
			continue
		}
		// One successful repository lookup is authoritative. Avoid retrying
		// equivalent .git/HTTPS/SSH spellings of the same repository.
		if latest, exists := tags[desiredMajor]; exists {
			return latest, nil
		}
		break
	}
	return "", fmt.Errorf("no stable Git tag found")
}

func (client *Client) repositories(ctx context.Context, base string) ([]string, error) {
	client.discoveryMu.Lock()
	if _, exists := client.unsupported[base]; exists {
		client.discoveryMu.Unlock()
		return nil, ErrUnsupportedVCS
	}
	if cached, exists := client.discovery[base]; exists {
		client.discoveryMu.Unlock()
		return cached, nil
	}
	client.discoveryMu.Unlock()

	candidates := inferredRepositories(base)
	discovered, discoverErr := client.goImportRepository(ctx, base)
	switch {
	case discoverErr == nil:
		candidates = append([]string{discovered}, candidates...)
	case errors.Is(discoverErr, ErrUnsupportedVCS):
		client.discoveryMu.Lock()
		client.unsupported[base] = struct{}{}
		client.discoveryMu.Unlock()
		return nil, ErrUnsupportedVCS
	}
	candidates = dedupe(candidates)

	client.discoveryMu.Lock()
	client.discovery[base] = candidates
	client.discoveryMu.Unlock()
	return candidates, nil
}

func (client *Client) goImportRepository(ctx context.Context, base string) (string, error) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(queryCtx, http.MethodGet, "https://"+base+"?go-get=1", nil)
	if err != nil {
		return "", err
	}
	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, downloadLimit))
		return "", fmt.Errorf("go-import endpoint returned %s", response.Status)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, downloadLimit))
	if err != nil {
		return "", err
	}
	content, err := goImportContent(body)
	if err != nil {
		return "", err
	}
	fields := strings.Fields(content)
	if len(fields) != 3 {
		return "", fmt.Errorf("invalid go-import record")
	}
	if !strings.EqualFold(fields[1], "git") {
		return "", ErrUnsupportedVCS
	}
	if moduleMatchesPrefix(base, fields[0]) {
		return fields[2], nil
	}
	return "", fmt.Errorf("go-import module prefix mismatch")
}

func goImportContent(body []byte) (string, error) {
	for _, tag := range metaTagPattern.FindAll(body, -1) {
		if !namePattern.Match(tag) {
			continue
		}
		match := contentPattern.FindSubmatch(tag)
		if match == nil {
			continue
		}
		for _, value := range match[1:] {
			if len(value) != 0 {
				return string(value), nil
			}
		}
	}
	return "", fmt.Errorf("go-import meta tag not found")
}

func (client *Client) tagsFor(ctx context.Context, repository string) (map[int]string, bool) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	client.tagsMu.Lock()
	if cached, exists := client.tags[repository]; exists {
		client.tagsMu.Unlock()
		return cached, true
	}
	client.tagsMu.Unlock()

	stdout, _, err := client.Runner.Run(queryCtx, "", runner.WithGitPromptDisabled(client.Env), "git", "ls-remote", "--tags", "--refs", repository)
	if err != nil {
		return nil, false
	}
	latest := LatestTags(stdout)

	client.tagsMu.Lock()
	client.tags[repository] = latest
	client.tagsMu.Unlock()
	return latest, true
}

// LatestTags maps each major version to its newest stable release tag.
func LatestTags(output string) map[int]string {
	latest := make(map[int]string)
	for _, line := range strings.Split(output, "\n") {
		reference := strings.TrimSpace(line)
		if reference == "" {
			continue
		}
		parts := strings.Fields(reference)
		if len(parts) != 2 || !strings.HasPrefix(parts[1], "refs/tags/") {
			continue
		}
		tag := strings.TrimPrefix(parts[1], "refs/tags/")
		if !version.IsRelease(tag) {
			continue
		}
		major := version.VersionMajor(tag)
		if existing, exists := latest[major]; !exists || version.Compare(tag, existing) > 0 {
			latest[major] = tag
		}
	}
	return latest
}

func inferredRepositories(base string) []string {
	slash := strings.Index(base, "/")
	if slash <= 0 || slash == len(base)-1 {
		return nil
	}
	host, path := base[:slash], strings.TrimSuffix(base[slash+1:], "/")
	path = strings.TrimSuffix(path, ".git")
	if path == "" {
		return nil
	}
	return []string{
		"https://" + host + "/" + path,
		"https://" + host + "/" + path + ".git",
		"ssh://git@" + host + "/" + path + ".git",
		"git@" + host + ":" + path + ".git",
	}
}

func moduleMatchesPrefix(modulePath, prefix string) bool {
	prefix = strings.TrimSuffix(prefix, "/")
	return modulePath == prefix || strings.HasPrefix(modulePath, prefix+"/")
}

func dedupe(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func majorFromPathMajor(value string) int {
	var major int
	if _, err := fmt.Sscanf(strings.TrimPrefix(strings.TrimPrefix(value, "/"), "v"), "%d", &major); err != nil {
		return 1
	}
	return major
}

// SortedMajors makes map iteration deterministic in tests.
func SortedMajors(tags map[int]string) []int {
	majors := make([]int, 0, len(tags))
	for major := range tags {
		majors = append(majors, major)
	}
	sort.Ints(majors)
	return majors
}
