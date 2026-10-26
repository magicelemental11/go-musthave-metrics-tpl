#!/bin/bash
set -e
out=${1:-bin}
version=${VERSION:-$(git describe --tags --always 2>/dev/null || true)}
date=$(date -u +%Y-%m-%dT%H:%M:%SZ)
commit=$(git rev-parse --short HEAD 2>/dev/null || true)
ldflags="-X main.buildVersion=$version -X main.buildDate=$date -X main.buildCommit=$commit"
mkdir -p "$out"
go build -ldflags "$ldflags" -o "$out/server" ./cmd/server
go build -ldflags "$ldflags" -o "$out/agent" ./cmd/agent
