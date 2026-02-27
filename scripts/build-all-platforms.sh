#!/bin/bash

# Multi-platform build script for ORLY relay
# Builds binaries for Linux, macOS, Windows, and Android
# NOTE: All builds use CGO_ENABLED=0 since p8k library uses purego (not CGO)
#       The library dynamically loads libsecp256k1 at runtime via purego

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
VERSION=$(cat pkg/version/version)
OUTPUT_DIR="$REPO_ROOT/build"
NOSTR_REPO_BASE_URL="https://git.nostrdev.com/mleku/nostr/raw/branch/main/crypto/p8k"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}ORLY Multi-Platform Build Script${NC}"
echo -e "${BLUE}========================================${NC}"
echo -e "Version: ${GREEN}${VERSION}${NC}"
echo -e "Output directory: ${OUTPUT_DIR}"
echo -e "${BLUE}Build mode: Pure Go with purego${NC}"
echo ""

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Function to build for a specific platform
build_platform() {
    local goos=$1
    local goarch=$2
    local platform_name=$3
    local binary_ext=$4
    
    echo -e "${YELLOW}Building for ${platform_name} (pure Go + purego)...${NC}"
    
    local output_name="orly-${VERSION}-${platform_name}${binary_ext}"
    local output_path="${OUTPUT_DIR}/${output_name}"
    
    # Build with CGO_ENABLED=0 (pure Go with purego)
    if CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch \
        go build -ldflags "-s -w -X main.version=${VERSION}" \
        -o "${output_path}" . 2>&1; then
        
        echo -e "${GREEN}✓ Built: ${output_name}${NC}"
        
        # Download appropriate runtime library from nostr repository
        case "$goos" in
            linux)
                if wget -q "${NOSTR_REPO_BASE_URL}/libsecp256k1.so" -O "${OUTPUT_DIR}/libsecp256k1-${platform_name}.so"; then
                    chmod +x "${OUTPUT_DIR}/libsecp256k1-${platform_name}.so"
                    echo -e "${GREEN}  ✓ Downloaded libsecp256k1.so (runtime optional)${NC}"
                else
                    echo -e "${YELLOW}  ⚠ Failed to download libsecp256k1.so (runtime optional)${NC}"
                fi
                ;;
            darwin)
                if wget -q "${NOSTR_REPO_BASE_URL}/libsecp256k1.dylib" -O "${OUTPUT_DIR}/libsecp256k1-${platform_name}.dylib"; then
                    chmod +x "${OUTPUT_DIR}/libsecp256k1-${platform_name}.dylib"
                    echo -e "${GREEN}  ✓ Downloaded libsecp256k1.dylib (runtime optional)${NC}"
                else
                    echo -e "${YELLOW}  ⚠ Failed to download libsecp256k1.dylib (runtime optional)${NC}"
                fi
                ;;
            windows)
                if wget -q "${NOSTR_REPO_BASE_URL}/libsecp256k1.dll" -O "${OUTPUT_DIR}/libsecp256k1-${platform_name}.dll"; then
                    chmod +x "${OUTPUT_DIR}/libsecp256k1-${platform_name}.dll"
                    echo -e "${GREEN}  ✓ Downloaded libsecp256k1.dll (runtime optional)${NC}"
                else
                    echo -e "${YELLOW}  ⚠ Failed to download libsecp256k1.dll (runtime optional)${NC}"
                fi
                ;;
            android)
                if wget -q "${NOSTR_REPO_BASE_URL}/libsecp256k1.so" -O "${OUTPUT_DIR}/libsecp256k1-${platform_name}.so"; then
                    chmod +x "${OUTPUT_DIR}/libsecp256k1-${platform_name}.so"
                    echo -e "${GREEN}  ✓ Downloaded libsecp256k1.so (runtime optional)${NC}"
                else
                    echo -e "${YELLOW}  ⚠ Failed to download libsecp256k1.so (runtime optional)${NC}"
                fi
                ;;
        esac
        
        # Create SHA256 checksum
        (cd "$OUTPUT_DIR" && sha256sum "$output_name" >> "SHA256SUMS-${VERSION}.txt")
        
        return 0
    else
        echo -e "${RED}✗ Failed to build for ${platform_name}${NC}"
        return 1
    fi
}

# Clean old builds
echo -e "${BLUE}Cleaning old builds...${NC}"
rm -f "${OUTPUT_DIR}/orly-"* "${OUTPUT_DIR}/libsecp256k1-"* "${OUTPUT_DIR}/SHA256SUMS-"*
echo ""

echo -e "${BLUE}Note: Pure Go builds work on all platforms${NC}"
echo -e "${BLUE}      libsecp256k1 is loaded at runtime if available (faster)${NC}"
echo -e "${BLUE}      Falls back to pure Go p256k1 if library not found${NC}"
echo ""

# Build for each platform
echo -e "${BLUE}Starting builds...${NC}"
echo ""

# Linux AMD64
build_platform "linux" "amd64" "linux-amd64" "" || true

# Linux ARM64
build_platform "linux" "arm64" "linux-arm64" "" || true

# macOS AMD64 (Intel)
build_platform "darwin" "amd64" "darwin-amd64" "" || true

# macOS ARM64 (Apple Silicon)
build_platform "darwin" "arm64" "darwin-arm64" "" || true

# Windows AMD64
build_platform "windows" "amd64" "windows-amd64" ".exe" || true

# Android builds
echo -e "${BLUE}Building for Android...${NC}"

# Android ARM64
build_platform "android" "arm64" "android-arm64" "" || true

# Android AMD64
build_platform "android" "amd64" "android-amd64" "" || true

echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "${GREEN}Build Summary${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# List all built binaries
if ls "${OUTPUT_DIR}"/orly-* 1> /dev/null 2>&1; then
    echo -e "${GREEN}Built binaries:${NC}"
    ls -lh "${OUTPUT_DIR}"/orly-* | awk '{print "  " $9 " (" $5 ")"}'
    echo ""
    
    if [ -f "${OUTPUT_DIR}/SHA256SUMS-${VERSION}.txt" ]; then
        echo -e "${GREEN}Checksums:${NC}"
        cat "${OUTPUT_DIR}/SHA256SUMS-${VERSION}.txt" | sed 's/^/  /'
        echo ""
    fi
    
    echo -e "${GREEN}Libraries:${NC}"
    ls -lh "${OUTPUT_DIR}"/libsecp256k1-* 2>/dev/null | awk '{print "  " $9 " (" $5 ")"}' || echo "  No libraries copied"
    echo ""
else
    echo -e "${RED}No binaries were built successfully${NC}"
    exit 1
fi

echo -e "${BLUE}========================================${NC}"
echo -e "${GREEN}Build completed!${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""
echo "Output directory: ${OUTPUT_DIR}"
echo ""
echo "To test a binary:"
echo "  Linux:   ./${OUTPUT_DIR}/orly-${VERSION}-linux-amd64"
echo "  macOS:   ./${OUTPUT_DIR}/orly-${VERSION}-darwin-arm64"
echo "  Windows: ./${OUTPUT_DIR}/orly-${VERSION}-windows-amd64.exe"
echo ""

