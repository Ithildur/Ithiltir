#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
node_root="${1:-$root/../Ithiltir-node}"
tools_dir="$(mktemp -d)"
trap 'rm -rf "$tools_dir"' EXIT
command -v protoc >/dev/null
GOBIN="$tools_dir" go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12
GOBIN="$tools_dir" go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2
for destination in "$root/internal/nodewire" "$node_root/internal/nodewire"; do
  mkdir -p "$destination"
  PATH="$tools_dir:$PATH" protoc -I "$root/protocol" \
    --go_out="$destination" --go_opt=paths=source_relative \
    --go-grpc_out="$destination" --go-grpc_opt=paths=source_relative \
    "$root/protocol/node.proto"
done
