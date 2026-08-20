# ADR-002: MCP server for interactive testing with Claude

## Status

Accepted

## Context

The lab needs a way to test ACM operations interactively against a live hub. Options considered:

1. **CLI only** — run commands manually and inspect output
2. **REST API** — expose operations as HTTP endpoints
3. **MCP server (stdio)** — expose operations as tools that Claude can call directly

## Decision

Implement an MCP server using `github.com/mark3labs/mcp-go` with stdio transport. The server exposes the same UC packages as the CLI, but as tools that Claude Code can call interactively. Long-running operations (provisioning, hibernation) return immediately and provide a polling tool for status.

## Consequences

- **Pro:** Claude can explore the ACM hub interactively — list clusters, create policies, deploy workloads
- **Pro:** Same Go packages serve CLI, MCP, and eventually the controller — no code duplication
- **Pro:** stdio transport requires no network setup, firewall rules, or authentication
- **Pro:** Natural fit for the conversational loop of testing and iterating
- **Con:** Tied to Claude Code as the client — not usable from other tools without adaptation
- **Con:** stdio is single-client — can't be shared across sessions

## Notes

The MCP server registers two levels of tools: low-level CRUD (create/get/delete individual resources) and high-level UC flows (provision-spoke, enforce-security). This mirrors how the ComputeRequest controller will work — high-level reconcile logic composed from low-level operations.
