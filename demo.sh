#!/bin/bash

# Demo script for Restricted Local Proxy
# This script demonstrates the proxy's functionality in both normal and discovery modes

set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
PROXY_PORT=9091
PROXY_BINARY="./restricted-proxy"
DISCOVERY_BINARY="./restricted-proxy-discovery"
LOGS_TO_CONFIG="./logs-to-config"

# Create/clean output directory
OUTPUT_DIR="demo_output"
rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"

echo -e "${BLUE}Output directory: $OUTPUT_DIR${NC}\n"

echo -e "${BLUE}==================================================================${NC}"
echo -e "${BLUE}Restricted Local Proxy - Demonstration${NC}"
echo -e "${BLUE}==================================================================${NC}\n"

# Check if binaries exist
if [[ ! -f "$PROXY_BINARY" ]]; then
    echo -e "${YELLOW}Building normal proxy...${NC}"
    make build
fi

if [[ ! -f "$DISCOVERY_BINARY" ]]; then
    echo -e "${YELLOW}Building discovery proxy...${NC}"
    make build-discovery
fi

if [[ ! -f "$LOGS_TO_CONFIG" ]]; then
    echo -e "${YELLOW}Building logs-to-config utility...${NC}"
    make build-tools
fi

# Cleanup function
cleanup() {
    echo -e "\n${YELLOW}Cleaning up...${NC}"
    if [[ -n "$PROXY_PID" ]] && kill -0 "$PROXY_PID" 2>/dev/null; then
        kill "$PROXY_PID" 2>/dev/null || true
        wait "$PROXY_PID" 2>/dev/null || true
    fi

    # Copy logs to output directory
    if [[ -f "$NORMAL_LOG" ]]; then
        cp "$NORMAL_LOG" "$OUTPUT_DIR/"
    fi
    if [[ -f "$DISCOVERY_LOG" ]]; then
        cp "$DISCOVERY_LOG" "$OUTPUT_DIR/"
    fi
    if [[ -f "$GENERATED_CONFIG" ]]; then
        cp "$GENERATED_CONFIG" "$OUTPUT_DIR/"
    fi

    # Save binary hashes
    echo -e "\n${YELLOW}Saving binary hashes...${NC}"
    sha256sum "$PROXY_BINARY" "$DISCOVERY_BINARY" > "$OUTPUT_DIR/binary_hashes.txt"

    # Save summary
    cat > "$OUTPUT_DIR/README.txt" <<EOF
Demo Run Summary
================
Timestamp: $(date)
Output Directory: $OUTPUT_DIR

Files:
- normal_mode.log: Logs from restricted mode proxy
- discovery_mode.log: Logs from discovery mode proxy
- generated_allowlist.yaml: Generated configuration from discovery logs
- binary_hashes.txt: SHA256 hashes of both binaries

Binaries Used:
- Normal mode: $PROXY_BINARY
- Discovery mode: $DISCOVERY_BINARY

Configuration:
- Listen port: $PROXY_PORT
- Allowlist: allowlist.yaml

See USAGE.md for more information.
EOF

    echo -e "${GREEN}Demo output saved to: $OUTPUT_DIR${NC}"
}

trap cleanup EXIT

# Function to test connection through proxy
test_connection() {
    local host=$1
    local expected_result=$2  # "success" or "blocked"

    echo -n "Testing connection to $host... "

    if curl -s -x http://localhost:$PROXY_PORT -m 5 "https://$host" > /dev/null 2>&1; then
        if [[ "$expected_result" == "success" ]]; then
            echo -e "${GREEN}✓ Allowed${NC}"
            return 0
        else
            echo -e "${RED}✗ Should have been blocked!${NC}"
            return 1
        fi
    else
        if [[ "$expected_result" == "blocked" ]]; then
            echo -e "${GREEN}✓ Blocked${NC}"
            return 0
        else
            echo -e "${RED}✗ Should have been allowed!${NC}"
            return 1
        fi
    fi
}

# Demo 1: Normal Mode
echo -e "${BLUE}==================================================================${NC}"
echo -e "${BLUE}Demo 1: Normal Mode (Restricted)${NC}"
echo -e "${BLUE}==================================================================${NC}\n"

# Log files
NORMAL_LOG="$OUTPUT_DIR/normal_mode.log"
DISCOVERY_LOG="$OUTPUT_DIR/discovery_mode.log"
GENERATED_CONFIG="$OUTPUT_DIR/generated_allowlist.yaml"

echo -e "${YELLOW}Starting proxy in normal mode on port $PROXY_PORT...${NC}"
$PROXY_BINARY --listen "localhost:$PROXY_PORT" > "$NORMAL_LOG" 2>&1 &
PROXY_PID=$!

# Wait for proxy to start
sleep 2

# Verify it's running
if ! kill -0 "$PROXY_PID" 2>/dev/null; then
    echo -e "${RED}Failed to start proxy! Check $NORMAL_LOG${NC}"
    cat "$NORMAL_LOG"
    exit 1
fi

echo -e "${GREEN}Proxy started (PID: $PROXY_PID)${NC}\n"

# Show current allowlist
echo -e "${YELLOW}Current allowlist:${NC}"
cat allowlist.yaml
echo ""

# Show the binary hash
echo -e "${YELLOW}Binary SHA256:${NC}"
sha256sum "$PROXY_BINARY"
echo ""

# Test allowed connections
echo -e "${YELLOW}Testing allowed destinations:${NC}"
test_connection "example.com" "success"
test_connection "httpbin.org" "success"

echo ""

# Test blocked connections
echo -e "${YELLOW}Testing blocked destinations:${NC}"
test_connection "twitter.com" "blocked"
test_connection "reddit.com" "blocked"

echo ""

# Show some log entries
echo -e "${YELLOW}Recent log entries:${NC}"
tail -5 "$NORMAL_LOG" | while IFS= read -r line; do
    echo "$line" | jq -r 'if .action == "blocked" then "\u001b[31m" + . + "\u001b[0m" elif .action == "allowed" then "\u001b[32m" + . + "\u001b[0m" else . end' 2>/dev/null || echo "$line"
done

echo ""

# Stop normal proxy
echo -e "${YELLOW}Stopping normal proxy...${NC}"
kill "$PROXY_PID"
wait "$PROXY_PID" 2>/dev/null || true
PROXY_PID=""

sleep 1

# Demo 2: Discovery Mode
echo -e "\n${BLUE}==================================================================${NC}"
echo -e "${BLUE}Demo 2: Discovery Mode${NC}"
echo -e "${BLUE}==================================================================${NC}\n"

echo -e "${YELLOW}Starting proxy in discovery mode on port $PROXY_PORT...${NC}"
$DISCOVERY_BINARY --listen "localhost:$PROXY_PORT" > "$DISCOVERY_LOG" 2>&1 &
PROXY_PID=$!

# Wait for proxy to start
sleep 2

# Verify it's running
if ! kill -0 "$PROXY_PID" 2>/dev/null; then
    echo -e "${RED}Failed to start discovery proxy! Check $DISCOVERY_LOG${NC}"
    cat "$DISCOVERY_LOG"
    exit 1
fi

echo -e "${GREEN}Discovery proxy started (PID: $PROXY_PID)${NC}\n"

# Show the binary hash (different from normal mode)
echo -e "${YELLOW}Discovery binary SHA256:${NC}"
sha256sum "$DISCOVERY_BINARY"
echo -e "${GREEN}Note: Different hash = different mode embedded in binary${NC}\n"

# Test various connections (all should be allowed in discovery mode)
echo -e "${YELLOW}Testing various destinations (all allowed in discovery mode):${NC}"
test_connection "example.com" "success"
test_connection "twitter.com" "success"
test_connection "reddit.com" "success"
test_connection "github.com" "success"

echo ""

# Show discovery log entries
echo -e "${YELLOW}Discovery log entries:${NC}"
tail -10 "$DISCOVERY_LOG" | while IFS= read -r line; do
    echo "$line" | jq '.' 2>/dev/null || echo "$line"
done

# Stop discovery proxy
echo -e "\n${YELLOW}Stopping discovery proxy...${NC}"
kill "$PROXY_PID"
wait "$PROXY_PID" 2>/dev/null || true
PROXY_PID=""

sleep 1

# Demo 3: Generate Config from Logs
echo -e "\n${BLUE}==================================================================${NC}"
echo -e "${BLUE}Demo 3: Generate Config from Discovery Logs${NC}"
echo -e "${BLUE}==================================================================${NC}\n"

echo -e "${YELLOW}Generating YAML config from discovery logs...${NC}"
$LOGS_TO_CONFIG -input "$DISCOVERY_LOG" -output "$GENERATED_CONFIG"

echo ""
echo -e "${YELLOW}Generated allowlist:${NC}"
cat "$GENERATED_CONFIG"

echo -e "\n${GREEN}You can now use this config by:${NC}"
echo "  1. Review: cat $GENERATED_CONFIG"
echo "  2. Replace: cp $GENERATED_CONFIG allowlist.yaml"
echo "  3. Rebuild: make build"
echo "  4. Deploy with new configuration (different SHA256)"

# Demo 4: Binary Verification
echo -e "\n${BLUE}==================================================================${NC}"
echo -e "${BLUE}Demo 4: Binary Verification${NC}"
echo -e "${BLUE}==================================================================${NC}\n"

echo -e "${YELLOW}Demonstrating SHA256 verification workflow:${NC}\n"

echo "1. Build both modes:"
echo "   - Normal mode: make build"
echo "   - Discovery mode: make build-discovery"
echo ""

echo "2. Record the SHA256 hashes in your 'known good list':"
sha256sum "$PROXY_BINARY" "$DISCOVERY_BINARY"
echo ""

echo "3. Start a proxy:"
VERIFY_LOG="$OUTPUT_DIR/verify_mode.log"
$PROXY_BINARY --listen "localhost:$PROXY_PORT" > "$VERIFY_LOG" 2>&1 &
PROXY_PID=$!
sleep 2

echo ""
echo "4. In production, verify running proxy:"
echo "   Command: lsof -ti :$PROXY_PORT | head -1 | xargs -I {} readlink -f /proc/{}/exe | xargs sha256sum"

if command -v lsof &> /dev/null; then
    echo "   Result:"
    RUNNING_BINARY=$(lsof -ti :$PROXY_PORT 2>/dev/null | head -1 | xargs -I {} readlink -f /proc/{}/exe 2>/dev/null || echo "")
    if [[ -n "$RUNNING_BINARY" ]]; then
        sha256sum "$RUNNING_BINARY"
        echo ""
        EXPECTED_HASH=$(sha256sum "$PROXY_BINARY" | cut -d' ' -f1)
        ACTUAL_HASH=$(sha256sum "$RUNNING_BINARY" | cut -d' ' -f1)
        if [[ "$EXPECTED_HASH" == "$ACTUAL_HASH" ]]; then
            echo -e "   ${GREEN}✓ Hash matches! Running proxy is verified.${NC}"
        else
            echo -e "   ${RED}✗ Hash mismatch! Unknown binary running!${NC}"
        fi
    else
        echo "   (lsof not available in this environment)"
    fi
else
    echo "   (lsof not available - would show hash of running binary)"
fi

kill "$PROXY_PID" 2>/dev/null || true
wait "$PROXY_PID" 2>/dev/null || true
PROXY_PID=""

echo ""
echo -e "${GREEN}==================================================================${NC}"
echo -e "${GREEN}Demo Complete!${NC}"
echo -e "${GREEN}==================================================================${NC}"
echo ""
echo "Key takeaways:"
echo "  • Normal mode enforces allowlist at the binary level"
echo "  • Discovery mode logs all connections for config generation"
echo "  • Each mode has a unique SHA256 hash"
echo "  • Single verification command confirms binary, port, and config"
echo "  • YAML config is embedded at build time (not runtime)"
echo ""
echo "See USAGE.md for detailed documentation."
