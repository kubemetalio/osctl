#!/bin/bash
set -e

BUILD_DIR="bin"
TARGET_SERVER_NAME="osctl"

CGO_ENABLED=0 GOOS="linux" GOARCH="amd64" go build -o $BUILD_DIR/$TARGET_SERVER_NAME cmd/osctl/main.go

if [ $? -ne 0 ]; then
    echo "Failed to build $TARGET_SERVER_NAME"
    exit 1
fi
echo "Building $TARGET_SERVER_NAME succeeded."
