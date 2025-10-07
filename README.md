# Restricted Local Proxy

This project seeks to build a single-file executable which provides forward proxy functionality without TLS termination, but enforces an allowlist of acceptable remote systems. This configuration will be included within the binary and not editable: it is not possible to run this tool with a different configuration than the one it was built with.

The reason for this is to enable a check like this:
```
# Find process and get SHA256
lsof -ti :9091 | xargs ps -o comm= -p | xargs shasum -a 256
```

The sha256sum can then be checked against a "known good list" and you can know several things from a single check:
1. The proxy is running on the intended port
1. The proxy is the right piece of code we intended; not something else
1. The configuration of the proxy is one of the blessed/approved configurations

## Claude's Understanding

This project aims to create a **single-file executable forward proxy** with the following characteristics:

**Core Functionality:**
- **Forward proxy functionality** - routes traffic to remote systems
- **No TLS termination** - passes encrypted traffic through without decrypting it
- **Hardcoded allowlist** - only permits connections to pre-approved remote systems
- **Immutable configuration** - the allowlist is compiled into the binary and cannot be changed at runtime

**Security Verification Model:**
The key innovation enables **integrity verification through SHA256 hashing**:
- Identify the process listening on a specific port (e.g., 9091)
- Hash the binary executable
- Compare the hash against a "known good list"

This single verification confirms:
- The correct proxy is running on the intended port
- The binary hasn't been tampered with
- The configuration (allowlist) is one of the approved versions

**Intent:**
This is a **defensive security control** designed to help organizations restrict outbound network access to only approved destinations, while providing a simple cryptographic method to verify both the proxy's integrity and its configuration in a single operation.

