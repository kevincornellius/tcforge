#!/bin/bash
set -e

REGISTRY="ghcr.io/kevincornellius"
TAG="${1:-dev}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$SCRIPT_DIR/.."

cd "$ROOT"

echo "==> Syncing Go workspace"
go work sync
go -C cli mod tidy
go -C api mod tidy
go -C judge mod tidy

echo ""
echo "==> Building CLI binary"
go build -C cli -o ../tcforge .
echo "    tcforge binary written to $(pwd)/tcforge"

echo ""
echo "==> Building web (npm)"
cd web && npm install --silent && npm run build
cd "$ROOT"

echo ""
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
echo "==> Building Docker images (tag: $TAG, built: $BUILD_TIME)"
docker build -t "$REGISTRY/tcforge-builder:$TAG" ./docker/builder
docker build --build-arg VERSION="$TAG" --build-arg BUILD_TIME="$BUILD_TIME" -t "$REGISTRY/tcforge-api:$TAG"   -f api/Dockerfile .
docker build --build-arg VERSION="$TAG" --build-arg BUILD_TIME="$BUILD_TIME" -t "$REGISTRY/tcforge-judge:$TAG" -f judge/Dockerfile .

echo ""
echo "==> Pushing images"
docker push "$REGISTRY/tcforge-builder:$TAG"
docker push "$REGISTRY/tcforge-api:$TAG"
docker push "$REGISTRY/tcforge-judge:$TAG"

echo ""
echo "Done. Binary at ./tcforge | Images pushed as :$TAG"
