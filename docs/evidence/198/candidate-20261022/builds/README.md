# Build provenance

The failed wrong-context MCP build log is stored as `mcp-build.log.base64` to preserve the original bytes, including Docker display whitespace, without adding whitespace errors to the source diff. It is not a successful build. Decode with Python base64 before viewing.

Original decoded SHA-256: `b2d3deff2e4a5e500fc2b3ec9c06ad94682d8278ef915e1e5d9107e9553d9b81`. The corrected build log and other original logs remain plain text.
