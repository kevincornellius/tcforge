#!/bin/bash
set -e

REGISTRY="ghcr.io/kevincornellius"
TAG="${1:-latest}"

echo "Building and pushing images with tag: $TAG"

docker build -t "$REGISTRY/tcforge-builder:$TAG" ./docker/builder
docker build -t "$REGISTRY/tcforge-api:$TAG"     -f api/Dockerfile .
docker build -t "$REGISTRY/tcforge-judge:$TAG"   -f judge/Dockerfile .

docker push "$REGISTRY/tcforge-builder:$TAG"
docker push "$REGISTRY/tcforge-api:$TAG"
docker push "$REGISTRY/tcforge-judge:$TAG"

echo "Done."
