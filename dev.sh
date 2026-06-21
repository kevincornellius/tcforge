#!/bin/bash
# dev.sh — rebuild tcforge components locally
# Usage:
#   ./dev.sh          # rebuild CLI binary only
#   ./dev.sh images   # rebuild all Docker images (builder, api, judge)
#   ./dev.sh all      # rebuild CLI + all images

set -e
cd "$(dirname "$0")"

TAG="${TCFORGE_TAG:-dev}"

build_cli() {
    echo "→ Building CLI binary..."
    go build -o /usr/local/bin/tcforge ./cli
    echo "  ✓ tcforge installed at /usr/local/bin/tcforge"
}

build_images() {
    BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    echo "→ Building Docker images (tag: $TAG, built: $BUILD_TIME)..."
    docker build -t ghcr.io/kevincornellius/tcforge-builder:$TAG ./docker/builder &
    docker build --build-arg VERSION=$TAG --build-arg BUILD_TIME=$BUILD_TIME -t ghcr.io/kevincornellius/tcforge-api:$TAG -f api/Dockerfile . &
    docker build --build-arg VERSION=$TAG --build-arg BUILD_TIME=$BUILD_TIME -t ghcr.io/kevincornellius/tcforge-judge:$TAG -f judge/Dockerfile . &
    wait
    echo "  ✓ builder, api, judge images built with tag :$TAG"
}

case "${1:-cli}" in
    cli)    build_cli ;;
    images) build_images ;;
    all)    build_cli && build_images ;;
    *)      echo "Usage: $0 [cli|images|all]"; exit 1 ;;
esac
