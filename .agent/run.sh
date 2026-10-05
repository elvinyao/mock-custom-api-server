#!/usr/bin/env bash
set -euo pipefail

PROJECT_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE="golang:1.26-bookworm"
case "${AGENT_RUNTIME:-${1:-}}" in
  node|npm|npx) IMAGE="node:22-bookworm" ;;
esac

# Publish the default server port only when explicitly requested.
docker_args=(--rm -i --init)
if [[ "$IMAGE" == golang:* ]]; then
  docker_args+=(-v mock-custom-api-go-mod:/go/pkg/mod -v mock-custom-api-go-build:/root/.cache/go-build)
else
  docker_args+=(-v mock-custom-api-npm:/root/.npm)
fi
if [[ -n "${AGENT_HTTP_PORT:-}" ]]; then
  docker_args+=(-p "127.0.0.1:${AGENT_HTTP_PORT}:8080")
fi

exec docker run "${docker_args[@]}" \
  -v "${PROJECT_ROOT}:/workspace" \
  -w /workspace \
  "${IMAGE}" \
  "$@"
