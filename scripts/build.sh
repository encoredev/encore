#!/usr/bin/env bash
set -euo pipefail

# Build script for Rencore (Encore fork)
# This script builds custom Encore CLI binaries with self-hosted platform URLs

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# Configuration (can be overridden via environment variables)
PLATFORM_API_URL="${RENCORE_PLATFORM_API_URL:-https://api.stagecraft.ing}"
DEVDASH_URL="${RENCORE_DEVDASH_URL:-https://devdash.stagecraft.ing}"
WEBDASH_URL="${RENCORE_WEBDASH_URL:-https://app.stagecraft.ing}"
DOCS_URL="${RENCORE_DOCS_URL:-https://docs.stagecraft.ing}"
VERSION="${RENCORE_VERSION:-v0.0.0-dev}"
OUTPUT_DIR="${OUTPUT_DIR:-$ROOT_DIR/dist}"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--version)
            VERSION="$2"
            shift 2
            ;;
        -o|--output)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        --platform-url)
            PLATFORM_API_URL="$2"
            shift 2
            ;;
        --webdash-url)
            WEBDASH_URL="$2"
            shift 2
            ;;
        --devdash-url)
            DEVDASH_URL="$2"
            shift 2
            ;;
        --docs-url)
            DOCS_URL="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  -v, --version VERSION     Version to build (default: v0.0.0-dev)"
            echo "  -o, --output DIR          Output directory (default: ./dist)"
            echo "  --platform-url URL        Platform API URL"
            echo "  --webdash-url URL         Web Dashboard URL"
            echo "  --devdash-url URL         Dev Dashboard URL"
            echo "  --docs-url URL            Documentation URL"
            echo "  -h, --help                Show this help message"
            echo ""
            echo "Environment variables:"
            echo "  RENCORE_PLATFORM_API_URL  Override platform API URL"
            echo "  RENCORE_WEBDASH_URL       Override web dashboard URL"
            echo "  RENCORE_DEVDASH_URL       Override dev dashboard URL"
            echo "  RENCORE_DOCS_URL          Override documentation URL"
            echo "  RENCORE_VERSION           Override version"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Run '$0 --help' for usage"
            exit 1
            ;;
    esac
done

echo "🏗️  Building Rencore CLI..."
echo ""
echo "   Version:      $VERSION"
echo "   Platform API: $PLATFORM_API_URL"
echo "   Web Dash:     $WEBDASH_URL"
echo "   Dev Dash:     $DEVDASH_URL"
echo "   Docs:         $DOCS_URL"
echo "   Output:       $OUTPUT_DIR"
echo ""

# Ensure version starts with 'v'
if [[ ! "$VERSION" =~ ^v ]]; then
    echo "❌ Error: Version must start with 'v' (e.g., v1.0.0)"
    exit 1
fi

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Build using the official build script
cd "$ROOT_DIR"

echo "📦 Running official build process..."
echo ""

go run ./pkg/encorebuild/cmd/make-release/ \
    -dst "$OUTPUT_DIR" \
    -v "$VERSION" \
    -publish-npm=false

echo ""
echo "✨ Build complete!"
echo ""
echo "📦 Artifacts:"
for artifact in "$OUTPUT_DIR/artifacts"/*.tar.gz; do
    if [ -f "$artifact" ]; then
        size=$(du -h "$artifact" | cut -f1)
        echo "   $(basename "$artifact") ($size)"
    fi
done
echo ""
echo "💡 To install locally:"
echo "   tar -xzf $OUTPUT_DIR/artifacts/encore-$VERSION-\$(uname -s | tr '[:upper:]' '[:lower:]')_\$(uname -m).tar.gz"
echo "   sudo mv encore /usr/local/bin/"
