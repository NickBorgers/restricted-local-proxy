# Usage Guide

## Overview

The Restricted Local Proxy now supports:
- **YAML-based configuration** embedded at build time
- **Structured JSON logging** for better observability
- **Discovery mode** for generating new configurations

## Building

### Normal Restricted Mode
```bash
make build
```
Builds `restricted-proxy` with allowlist enforcement enabled.

### Discovery Mode
```bash
make build-discovery
```
Builds `restricted-proxy-discovery` which logs all connection attempts without blocking.

### Log Analysis Tool
```bash
make build-tools
```
Builds the `logs-to-config` utility for converting discovery logs to YAML configs.

### Build All
```bash
make build-both build-tools
```

## Configuration

Edit `allowlist.yaml` to configure allowed destinations:

```yaml
# Allowlist configuration for restricted local proxy
# Format: hostname:port or just hostname (allows any port)
allowlist:
  - example.com
  - example.org
  - www.google.com:443
  - api.github.com:443
```

**Important:** The YAML file is embedded into the binary at compile time. To use a new configuration:
1. Edit `allowlist.yaml`
2. Rebuild the binary with `make build`
3. The new binary will have a different SHA256 hash

## Running

### Command Line Options

The proxy supports the following flags (similar to ghostunnel):

- `--listen <address>`: Address to listen on (default: `localhost:9091`)
  - Examples: `localhost:8080`, `:9091`, `0.0.0.0:3128`

### Normal Mode
```bash
# Use default port (localhost:9091)
./restricted-proxy

# Specify custom listen address
./restricted-proxy --listen localhost:8080

# Listen on all interfaces
./restricted-proxy --listen :9091
```
Logs are output as JSON to stdout.

### Discovery Mode
```bash
# Use default port
./restricted-proxy-discovery > discovery.log

# Specify custom listen address
./restricted-proxy-discovery --listen localhost:8080 > discovery.log
```
Run this version to collect connection attempts. All connections are allowed and logged.

## Verifying Configuration

As described in the README, you can verify the running binary and its configuration:

```bash
# Find process and get SHA256
lsof -ti :9091 | head -1 | xargs -I {} readlink -f /proc/{}/exe | xargs sha256sum
```

Compare the hash against your known good list:
- Normal mode binary: `f4a9589e5e2cc221506641191d17fe769b5c9a307895e608b01c2237c9db0f61`
- Discovery mode binary: `262aa011c0fc5ab3337af33f6c04947183dc755e87dfc19b985f8e8f7b74ae62`

**Note:** These hashes will change if you modify `allowlist.yaml` or rebuild with a different Go version.

## Log Format

All logs are structured JSON with the following format:

```json
{
  "timestamp": "2025-10-07T19:00:00Z",
  "level": "INFO",
  "event": "connection_attempt",
  "destination": "example.com:443",
  "action": "allowed"
}
```

### Common Events

- `proxy_starting` - Proxy server has started
- `connection_attempt` - Client attempted connection (action: allowed/blocked/allowed_discovery)
- `connection_closed` - Connection terminated
- `connection_failed` - Failed to connect to destination

### Log Levels

- `INFO` - Normal operational events
- `DEBUG` - Detailed information (e.g., allowlist entries)
- `WARNING` - Unexpected situations
- `ERROR` - Error conditions

## Generating Configuration from Discovery Logs

After running in discovery mode and collecting logs:

```bash
# Convert discovery logs to YAML config
./logs-to-config -input discovery.log -output new-allowlist.yaml

# Review the generated config
cat new-allowlist.yaml

# Replace the current config
mv new-allowlist.yaml allowlist.yaml

# Rebuild with new config
make build
```

The tool extracts all unique destinations from `connection_attempt` events and generates a sorted YAML config.

## Example Workflow

1. **Initial deployment with known destinations:**
   ```bash
   # Edit allowlist.yaml with known destinations
   make build
   ./restricted-proxy --listen localhost:9091 > proxy.log &
   ```

2. **Discover new destinations:**
   ```bash
   # Build and run discovery mode
   make build-discovery
   ./restricted-proxy-discovery --listen localhost:9091 > discovery.log &

   # Let it run for a period (hours/days)
   # Stop the proxy when done

   # Generate new config
   ./logs-to-config -input discovery.log -output new-allowlist.yaml
   ```

3. **Review and deploy new config:**
   ```bash
   # Review destinations
   cat new-allowlist.yaml

   # Update and rebuild
   mv new-allowlist.yaml allowlist.yaml
   make build

   # Verify new hash
   sha256sum restricted-proxy

   # Deploy with new configuration
   ```

## Security Notes

- Discovery mode should **only be run in controlled environments** for configuration generation
- Always review the generated allowlist before deploying to production
- Keep track of SHA256 hashes for your approved configurations
- Different configurations produce different binary hashes - this is the security feature
