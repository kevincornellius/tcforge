#!/bin/bash
set -e

REGISTRY="ghcr.io/kevincornellius"
TAG="${1:-latest}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$SCRIPT_DIR/.."

cd "$ROOT"

echo "==> Building CLI binary"
go build -C cli -o ../tcforge .
echo "    tcforge binary written to $(pwd)/tcforge"

echo ""
echo "==> Building web (npm)"
cd web && npm install --silent && npm run build
cd "$ROOT"

echo ""
echo "==> Building Docker images (tag: $TAG)"
docker build -t "$REGISTRY/tcforge-builder:$TAG" ./docker/builder
docker build -t "$REGISTRY/tcforge-api:$TAG"     -f api/Dockerfile .
docker build -t "$REGISTRY/tcforge-judge:$TAG"   -f judge/Dockerfile .

echo ""
echo "==> Pushing images"
docker push "$REGISTRY/tcforge-builder:$TAG"
docker push "$REGISTRY/tcforge-api:$TAG"
docker push "$REGISTRY/tcforge-judge:$TAG"

echo ""
echo "Done. Binary at ./tcforge | Images pushed as :$TAG"
