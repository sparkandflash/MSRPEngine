#!/bin/bash

# Navigate to the directory containing the build script
cd "$(dirname "$0")"

SAMPLE=$1
if [ -z "$SAMPLE" ]; then
    echo "Usage: ./build.sh <sample_name>"
    echo "Available samples:"
    ls -1 samples/
    exit 1
fi

SAMPLE_DIR="samples/$SAMPLE"

if [ ! -d "$SAMPLE_DIR" ]; then
    echo "Sample directory $SAMPLE_DIR does not exist."
    exit 1
fi

if [ ! -f "$SAMPLE_DIR/personality.txt" ]; then
    echo "Error: $SAMPLE_DIR/personality.txt not found!"
    exit 1
fi

# Hardcode the personality prompt for this specific binary
echo "Injecting personality for $SAMPLE..."
cp "$SAMPLE_DIR/personality.txt" src/prompts/personality.txt

BUILD_DIR="build/$SAMPLE"
mkdir -p "$BUILD_DIR"

echo "Building $SAMPLE binary..."
go build -o "$BUILD_DIR/$SAMPLE" main.go

if [ $? -eq 0 ]; then
    echo "Copying .env to build directory..."
    cp "$SAMPLE_DIR/.env" "$BUILD_DIR/" 2>/dev/null || true
    echo "Successfully compiled standalone binary to $BUILD_DIR/"
else
    echo "Build failed!"
    exit 1
fi
