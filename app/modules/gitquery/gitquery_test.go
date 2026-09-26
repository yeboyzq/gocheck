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

package gitquery

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestLatestTagsFiltersUnstableAndPeeledTags(t *testing.T) {
	output := `1000000000000000000000000000000000000001	refs/tags/v1.0.0
1000000000000000000000000000000000000002	refs/tags/v1.10.0
1000000000000000000000000000000000000003	refs/tags/v1.2.0-rc.1
1000000000000000000000000000000000000004	refs/tags/release
1000000000000000000000000000000000000005	refs/tags/v2.0.0+incompatible
1000000000000000000000000000000000000006	refs/tags/v2.1.0
`

	tags := LatestTags(output)
	majors := SortedMajors(tags)
	if len(majors) != 2 || majors[0] != 1 || majors[1] != 2 {
		t.Fatalf("majors = %v, want [1 2]", majors)
	}
	if tags[1] != "v1.10.0" {
		t.Fatalf("latest v1 = %q, want v1.10.0", tags[1])
	}
	if tags[2] != "v2.1.0" {
		t.Fatalf("latest v2 = %q, want v2.1.0", tags[2])
	}
}

func TestGoImportContentSupportsAttributeOrders(t *testing.T) {
	content, err := goImportContent([]byte(`<html><head>
		<meta name="go-import" content="example.com/module git https://git.example.com/module.git">
		<meta content="example.com/other git https://git.example.com/other.git" name="go-import">
	</head></html>`))
	if err != nil {
		t.Fatalf("goImportContent() error = %v", err)
	}
	if content != "example.com/module git https://git.example.com/module.git" {
		t.Fatalf("goImportContent() = %q", content)
	}
}

func TestRepositoriesRejectsNonGitVCS(t *testing.T) {
	client := NewClient(nil)
	client.HTTPClient = &http.Client{Transport: staticTransport{
		body: `<meta name="go-import" content="example.com/module hg https://hg.example.com/module">`,
	}}

	_, err := client.repositories(context.Background(), "example.com/module")
	if !errors.Is(err, ErrUnsupportedVCS) {
		t.Fatalf("repositories() error = %v, want ErrUnsupportedVCS", err)
	}
	if _, err := client.repositories(context.Background(), "example.com/module"); !errors.Is(err, ErrUnsupportedVCS) {
		t.Fatalf("cached repositories() error = %v, want ErrUnsupportedVCS", err)
	}
}

type staticTransport struct {
	body string
}

func (transport staticTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return &http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(transport.body)),
		Header:     make(http.Header),
		Request:    request,
	}, nil
}
