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

