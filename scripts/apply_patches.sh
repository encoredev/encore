#!/usr/bin/env bash
set -euo pipefail

# Apply patches script for Rencore (Encore fork)
# This script applies all patches in the patches/ directory to the upstream codebase

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
PATCHES_DIR="$ROOT_DIR/patches"

echo "🔧 Applying Rencore patches..."
echo "   Root: $ROOT_DIR"
echo "   Patches: $PATCHES_DIR"
echo ""

# Check if patches directory exists
if [ ! -d "$PATCHES_DIR" ]; then
    echo "❌ Error: patches/ directory not found"
    exit 1
fi

# Count patches
patch_count=$(find "$PATCHES_DIR" -name "*.patch" | wc -l | tr -d ' ')
if [ "$patch_count" -eq 0 ]; then
    echo "⚠️  No patches found in patches/ directory"
    exit 0
fi

echo "📦 Found $patch_count patch(es) to apply"
echo ""

# Apply each patch in order
success_count=0
failed_count=0

for patch_file in "$PATCHES_DIR"/*.patch; do
    patch_name=$(basename "$patch_file")
    echo "   Applying: $patch_name"

    if git apply --check "$patch_file" 2>/dev/null; then
        git apply "$patch_file"
        echo "   ✅ Applied: $patch_name"
        ((success_count++))
    else
        echo "   ❌ Failed: $patch_name"
        echo "      Run 'git apply --check $patch_file' for details"
        ((failed_count++))
    fi
    echo ""
done

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📊 Patch Summary:"
echo "   ✅ Success: $success_count"
echo "   ❌ Failed:  $failed_count"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [ "$failed_count" -gt 0 ]; then
    echo ""
    echo "⚠️  Some patches failed to apply. This may be due to:"
    echo "   - Upstream changes conflicting with patches"
    echo "   - Patches already applied"
    echo "   - Out-of-date patch files"
    echo ""
    echo "💡 To resolve:"
    echo "   1. Check upstream.ref for the pinned commit"
    echo "   2. Run: git checkout \$(cat upstream.ref)"
    echo "   3. Then re-run this script"
    exit 1
fi

echo ""
echo "✨ All patches applied successfully!"
