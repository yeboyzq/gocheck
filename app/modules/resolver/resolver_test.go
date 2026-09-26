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

package resolver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/yeboyzq/gocheck/app/modules/gitquery"
	"github.com/yeboyzq/gocheck/app/modules/modfile"
)

type fakeGoRunner struct {
	versions map[string]string
	mu       sync.Mutex
	Calls    int
}

func (fake *fakeGoRunner) Run(ctx context.Context, dir string, env []string, name string, args ...string) (string, string, error) {
	fake.mu.Lock()
	fake.Calls++
	fake.mu.Unlock()
	if name != "go" || len(args) == 0 {
		return "", "unexpected command", errors.New("unexpected command")
	}
	modulePath := strings.TrimSuffix(args[len(args)-1], "@latest")
	version, exists := fake.versions[modulePath]
	if !exists {
		return "", "module not found", errors.New("module not found")
	}
	return fmt.Sprintf("{\"Path\":%q,\"Version\":%q}\n", modulePath, version), "", nil
}

type fakeGitRunner struct {
	tags string
}

func (fake fakeGitRunner) Run(ctx context.Context, dir string, env []string, name string, args ...string) (string, string, error) {
	if name != "git" || len(args) == 0 {
		return "", "unexpected command", errors.New("unexpected command")
	}
	return fake.tags, "", nil
}

type failingTransport struct{}

func (failingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return nil, errors.New("network unavailable")
}

func testResolver(goVersions map[string]string, gitTags string, concurrency int) Resolver {
	gitClient := gitquery.NewClient(nil)
	gitClient.HTTPClient = &http.Client{Transport: failingTransport{}}
	gitClient.Runner = fakeGitRunner{tags: gitTags}
	return Resolver{
		Concurrency: concurrency,
		GoList:      &GoList{Runner: &fakeGoRunner{versions: goVersions}, Env: nil},
		Git:         gitClient,
	}
}

func TestResolveUpdateWithoutMajor(t *testing.T) {
	resolver := testResolver(
		map[string]string{"example.com/module": "v1.2.0"},
		"commit refs/tags/v1.2.0\n",
		2,
	)
	results := resolver.Resolve(context.Background(), []modfile.Dependency{
		{Path: "example.com/module", Current: "v1.0.0"},
	})
	if len(results) != 1 {
		t.Fatalf("results length = %d", len(results))
	}
	result := results[0]
	if !result.UpdateAvailable || result.Major || result.Error != "" {
		t.Fatalf("result = %+v", result)
	}
	if result.Latest != "v1.2.0" {
		t.Fatalf("Latest = %q", result.Latest)
	}
}

func TestResolveMajorUpdate(t *testing.T) {
	resolver := testResolver(
		map[string]string{"example.com/module": "v1.2.0"},
		"commit refs/tags/v1.2.0\ncommit refs/tags/v3.1.0\n",
		2,
	)
	results := resolver.Resolve(context.Background(), []modfile.Dependency{
		{Path: "example.com/module", Current: "v1.0.0"},
	})
	result := results[0]
	if !result.UpdateAvailable || !result.Major || result.Latest != "v3.1.0" || result.Error != "" {
		t.Fatalf("result = %+v", result)
	}
}

func TestResolveNoUpdate(t *testing.T) {
	resolver := testResolver(
		map[string]string{"example.com/module": "v1.2.0"},
		"commit refs/tags/v1.2.0\n",
		2,
	)
	results := resolver.Resolve(context.Background(), []modfile.Dependency{
		{Path: "example.com/module", Current: "v1.2.0"},
	})
	if results[0].UpdateAvailable || results[0].Error != "" {
		t.Fatalf("result = %+v", results[0])
	}
}

func TestResolveBuildMetadataIsNotAnUpdate(t *testing.T) {
	resolver := testResolver(
		map[string]string{"example.com/module": "v2.0.0"},
		"commit refs/tags/v2.0.0\n",
		1,
	)
	results := resolver.Resolve(context.Background(), []modfile.Dependency{
		{Path: "example.com/module", Current: "v2.0.0+incompatible"},
	})
	if results[0].UpdateAvailable || results[0].Error != "" {
		t.Fatalf("result = %+v", results[0])
	}
}

func TestResolveV0ToV1IsNotAMajorModuleUpdate(t *testing.T) {
	resolver := testResolver(
		map[string]string{"example.com/module": "v1.0.0"},
		"commit refs/tags/v1.0.0\n",
		1,
	)
	results := resolver.Resolve(context.Background(), []modfile.Dependency{
		{Path: "example.com/module", Current: "v0.1.0"},
	})
	if !results[0].UpdateAvailable || results[0].Major || results[0].Error != "" {
		t.Fatalf("result = %+v", results[0])
	}
}

func TestResolveLatestPreReleaseIsNotAQueryError(t *testing.T) {
	resolver := testResolver(
		map[string]string{"github.com/swaggo/swag/v2": "v2.0.0-rc6"},
		"",
		1,
	)
	results := resolver.Resolve(context.Background(), []modfile.Dependency{
		{Path: "github.com/swaggo/swag/v2", Current: "v2.0.0-rc6"},
	})
	result := results[0]
	if result.Error != "" || result.UpdateAvailable || !result.PreRelease || result.Latest != "v2.0.0-rc6" {
		t.Fatalf("result = %+v", result)
	}
}

func TestResolveNewerPreReleaseIsCompared(t *testing.T) {
	resolver := testResolver(
		map[string]string{"go.yaml.in/yaml/v4": "v4.0.0-rc.6"},
		"",
		1,
	)
	results := resolver.Resolve(context.Background(), []modfile.Dependency{
		{Path: "go.yaml.in/yaml/v4", Current: "v4.0.0-rc.5"},
	})
	result := results[0]
	if result.Error != "" || !result.UpdateAvailable || !result.PreRelease || result.Latest != "v4.0.0-rc.6" {
		t.Fatalf("result = %+v", result)
	}
}

func TestResolveIgnoresPreReleaseOnlyMajorCandidate(t *testing.T) {
	resolver := testResolver(
		map[string]string{
			"example.com/module":    "v1.2.0",
			"example.com/module/v2": "v2.0.0-rc.1",
		},
		"commit refs/tags/v1.2.0\n",
		1,
	)
	results := resolver.Resolve(context.Background(), []modfile.Dependency{
		{Path: "example.com/module", Current: "v1.1.0"},
	})
	if results[0].Latest != "v1.2.0" || results[0].Major || results[0].Error != "" {
		t.Fatalf("result = %+v", results[0])
	}
}

func TestResolveLocalReplacementDoesNotQueryNetwork(t *testing.T) {
	fake := &fakeGoRunner{}
	gitClient := gitquery.NewClient(nil)
	gitClient.HTTPClient = &http.Client{Transport: failingTransport{}}
	gitClient.Runner = fakeGitRunner{}
	resolver := Resolver{
		Concurrency: 1,
		GoList:      &GoList{Runner: fake},
		Git:         gitClient,
	}
	results := resolver.Resolve(context.Background(), []modfile.Dependency{
		{Path: "example.com/module", Current: "v1.0.0", Replacement: &modfile.Replacement{Path: "../local", Local: true}},
	})
	if fake.Calls != 0 {
		t.Fatalf("go command calls = %d, want 0", fake.Calls)
	}
	if results[0].Current != "local" || !results[0].Replace || results[0].UpdateAvailable || results[0].Error != "" {
		t.Fatalf("result = %+v", results[0])
	}
}

func TestResolveModuleReplacementUsesReplacementModule(t *testing.T) {
	resolver := testResolver(
		map[string]string{"example.com/replacement": "v1.1.0"},
		"commit refs/tags/v1.1.0\n",
		1,
	)
	results := resolver.Resolve(context.Background(), []modfile.Dependency{
		{
			Path:        "example.com/original",
			Current:     "v0.1.0",
			Replacement: &modfile.Replacement{Path: "example.com/replacement", Version: "v1.0.0"},
		},
	})
	result := results[0]
	if !result.Replace || !result.UpdateAvailable || result.Latest != "v1.1.0" || result.Error != "" {
		t.Fatalf("result = %+v", result)
	}
}

func TestResolveFailureAndInvalidCurrentVersion(t *testing.T) {
	resolver := testResolver(nil, "", 2)
	results := resolver.Resolve(context.Background(), []modfile.Dependency{
		{Path: "example.com/unavailable", Current: "v1.0.0"},
		{Path: "example.com/invalid", Current: "development"},
	})
	if results[0].Error == "" {
		t.Fatalf("unavailable result = %+v", results[0])
	}
	if results[1].Error == "" {
		t.Fatalf("invalid result = %+v", results[1])
	}
}
