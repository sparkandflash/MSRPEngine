#!/bin/bash

# Navigate to the directory containing the build script
cd "$(dirname "$0")"

BIN_NAME=$1
if [ -z "$BIN_NAME" ]; then
    echo "Usage: ./build.sh <binary_name>"
    exit 1
fi

BUILD_DIR="build"
mkdir -p "$BUILD_DIR"

echo "Building $BIN_NAME binary..."
go build -o "$BUILD_DIR/$BIN_NAME" main.go

if [ $? -eq 0 ]; then
    echo "Successfully compiled standalone binary to $BUILD_DIR/$BIN_NAME"
else
    echo "Build failed!"
    exit 1
fi
