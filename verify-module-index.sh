#!/usr/bin/env bash
# Copyright (c) 2026 gocheck
# gocheck is licensed under Mulan PSL v2.
# You can use this software according to the terms and conditions of the Mulan PSL v2.
# You may obtain a copy of Mulan PSL v2 at:
#         http://license.coscl.org.cn/MulanPSL2
# THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND,
# EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT,
# MERCHANTABILITY OR FIT FOR A PARTICULAR PURPOSE.
# See the Mulan PSL v2 for more details.

set -Eeuo pipefail

readonly EXPECTED_MODULE="github.com/yeboyzq/gocheck"
readonly GITHUB_REMOTE_URL="${GITHUB_REMOTE_URL:-https://github.com/yeboyzq/gocheck.git}"
PROXY_BASE="${PROXY_BASE:-https://proxy.golang.org}"
readonly PROXY_BASE
readonly PKG_GO_DEV_TIMEOUT_SECONDS="${PKG_GO_DEV_TIMEOUT_SECONDS:-300}"
readonly PKG_GO_DEV_RETRY_SECONDS="${PKG_GO_DEV_RETRY_SECONDS:-10}"

temp_dir=""

cleanup() {
    if [[ -n "$temp_dir" ]]; then
        rm -rf "$temp_dir"
    fi
}

fail() {
    printf 'FAIL: %s\n' "$*" >&2
    exit 1
}

on_exit() {
    local status=$?
    cleanup
    if (( status != 0 )); then
        printf '\nRESULT: FAILED\n' >&2
    fi
    exit "$status"
}
trap on_exit EXIT

usage() {
    cat <<'EOF'
Usage:
  ./verify-module-index.sh <version>

Example:
  ./verify-module-index.sh v0.1.0

Environment:
  PROXY_BASE                 Go module proxy used for verification and install.
                             Default: https://proxy.golang.org
  PKG_GO_DEV_TIMEOUT_SECONDS  Maximum wait for pkg.go.dev indexing. Default: 300
  PKG_GO_DEV_RETRY_SECONDS    Interval between pkg.go.dev checks. Default: 10
  GITHUB_REMOTE_URL           Git URL used to verify the published tag.

The script:
  1. Verifies a stable semantic version and its GitHub tag.
  2. Requests the configured Go module proxy to fetch the module.
  3. Validates .info, .mod, and version-list responses.
  4. Installs the exact version from the configured proxy in a clean temporary Go environment.
  5. Runs the installed gocheck command.
  6. Requests and verifies the pkg.go.dev version page.
EOF
}

if [[ $# -eq 1 && ( "$1" == "-h" || "$1" == "--help" ) ]]; then
    usage
    exit 0
fi
if [[ $# -ne 1 ]]; then
    usage >&2
    fail "exactly one version argument is required"
fi

version="$1"
[[ "$version" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]] || {
    printf 'Usage: %s\n' "$*" >&2
    fail "version must be a stable semantic version such as v0.1.0; pre-release tags are rejected because @latest prefers stable releases"
}

module="$(awk '/^module[[:space:]]/ { print $2; exit }' go.mod)"
[[ "$module" == "$EXPECTED_MODULE" ]] || fail "go.mod module is $module; expected $EXPECTED_MODULE"

for command in curl go git; do
    command -v "$command" >/dev/null 2>&1 || fail "required command is not installed: $command"
done

printf '== Configuration ==\n'
printf 'Module: %s\n' "$module"
printf 'Version: %s\n' "$version"
printf 'GitHub tag URL: %s\n' "$GITHUB_REMOTE_URL"
printf 'Proxy: %s\n' "$PROXY_BASE"
printf '\n'

printf '[1/6] Verifying GitHub tag...\n'
remote_tags="$(git ls-remote --tags "$GITHUB_REMOTE_URL" "refs/tags/$version")" || fail "could not query GitHub tags"
[[ -n "$remote_tags" ]] || fail "GitHub does not contain tag $version; push it first"

remote_commit="$(awk -v tag="refs/tags/$version" '$2 == tag { print $1; exit }' <<<"$remote_tags")"
peeled_commit="$(awk -v tag="refs/tags/$version^{}" '$2 == tag { print $1; exit }' <<<"$remote_tags")"
[[ -n "$peeled_commit" ]] && remote_commit="$peeled_commit"

if git rev-parse -q --verify "refs/tags/$version^{commit}" >/dev/null 2>&1; then
    local_commit="$(git rev-parse "${version}^{commit}")"
    [[ "$local_commit" == "$remote_commit" ]] || fail "local tag $version points to $local_commit, but GitHub tag points to $remote_commit"
    printf 'PASS GitHub tag exists and matches local commit %s\n' "$remote_commit"
else
    printf 'PASS GitHub tag exists at commit %s (no matching local tag)\n' "$remote_commit"
fi

temp_dir="$(mktemp -d "${TMPDIR:-/tmp}/gocheck-index-verify-XXXXXX")"

request_proxy() {
    local endpoint="$1"
    local output="$2"
    if ! curl --fail --location --silent --show-error \
        --retry 5 --retry-delay 2 --retry-connrefused \
        --output "$output" \
        "$PROXY_BASE/$module/@v/$endpoint"; then
        printf '\nDiagnostic:\n' >&2
        printf 'Could not connect to the Go module proxy: %s\n' "$PROXY_BASE" >&2
        printf 'This is a local network connectivity failure, not proof that the module is unavailable.\n' >&2
        printf 'You can test another proxy, for example:\n' >&2
        printf '  PROXY_BASE=https://goproxy.cn ./verify-module-index.sh %s\n' "$version" >&2
        printf 'To verify the official proxy and pkg.go.dev from a network with access, run the script on GitHub Actions or another host.\n' >&2
        fail "proxy request failed: $endpoint"
    fi
}

printf '\n[2/6] Requesting module metadata from %s...\n' "$PROXY_BASE"
info_file="$temp_dir/info.json"
mod_file="$temp_dir/mod"
list_file="$temp_dir/list"
request_proxy "$version.info" "$info_file"
grep -Eq "\"Version\"[[:space:]]*:[[:space:]]*\"$version\"" "$info_file" || fail "proxy .info response does not contain $version"
printf 'PASS proxy .info contains %s\n' "$version"

printf '\n[3/6] Validating module metadata and version list...\n'
request_proxy "$version.mod" "$mod_file"
grep -Eq "^module[[:space:]]+$module$" "$mod_file" || fail "proxy .mod file has an unexpected module path"
printf 'PASS proxy .mod module path is %s\n' "$module"

request_proxy "list" "$list_file"
tr '[:space:]' '\n' <"$list_file" | grep -Fqx "$version" || fail "proxy version list does not contain $version"
printf 'PASS proxy version list contains %s\n' "$version"

printf '\n[4/6] Installing exact version in a clean Go environment...\n'
install_bin="$temp_dir/bin"
go_cache="$temp_dir/go-cache"
mod_cache="$temp_dir/go-mod-cache"
mkdir -p "$install_bin" "$go_cache" "$mod_cache"

GOENV=off \
GOFLAGS= \
GOPROXY="$PROXY_BASE,direct" \
GOSUMDB=sum.golang.org \
GOPRIVATE= \
GONOSUMDB= \
GONOSUMCHECK= \
GOCACHE="$go_cache" \
GOMODCACHE="$mod_cache" \
GOBIN="$install_bin" \
GOTOOLCHAIN=local \
go install "$module@$version" || fail "clean go install failed"

installed_binary="$install_bin/gocheck"
[[ -x "$installed_binary" ]] || fail "installed gocheck binary was not found"
printf 'PASS go install installed %s\n' "$installed_binary"

printf '\n[5/6] Running installed gocheck...\n'
help_output="$("$installed_binary" --help)"
grep -Fq "Check direct Go module dependencies for available updates" <<<"$help_output" || fail "installed gocheck --help returned unexpected output"
printf 'PASS installed gocheck --help works\n'

printf '\n[6/6] Requesting and verifying pkg.go.dev documentation...\n'
pkg_page="$temp_dir/pkg-go-dev.html"
pkg_url="https://pkg.go.dev/$module@$version"
deadline=$((SECONDS + PKG_GO_DEV_TIMEOUT_SECONDS))
indexed=false

while (( SECONDS < deadline )); do
    http_code="$(
        curl --location --silent --show-error --output "$pkg_page" \
            --write-out '%{http_code}' "$pkg_url" 2>/dev/null || true
    )"
    if [[ "$http_code" == "200" ]] &&
        grep -Fq "$module" "$pkg_page" &&
        grep -Fq "$version" "$pkg_page"; then
        indexed=true
        break
    fi
    printf 'WAIT pkg.go.dev is not ready yet (HTTP %s); retrying in %ss...\n' \
        "${http_code:-unknown}" "$PKG_GO_DEV_RETRY_SECONDS"
    sleep "$PKG_GO_DEV_RETRY_SECONDS"
done

if [[ "$indexed" != true ]]; then
    fail "pkg.go.dev did not expose $pkg_url within ${PKG_GO_DEV_TIMEOUT_SECONDS}s; indexing may still complete later"
fi

printf 'PASS pkg.go.dev version page is available: %s\n' "$pkg_url"
printf '\n== Verification Summary ==\n'
printf 'PASS GitHub tag: %s\n' "$version"
printf 'PASS proxy.golang.org metadata: %s\n' "$version"
printf 'PASS proxy.golang.org version list: %s\n' "$version"
printf 'PASS clean go install: %s@%s\n' "$module" "$version"
printf 'PASS installed binary smoke test: gocheck --help\n'
printf 'PASS pkg.go.dev documentation: %s\n' "$pkg_url"
printf '\nRESULT: SUCCESS\n'
