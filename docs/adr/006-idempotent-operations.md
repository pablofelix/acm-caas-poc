# ADR-006: Idempotent operations

## Status

Accepted

## Context

The UC packages will be consumed by three callers: CLI (human retries), MCP server (Claude may retry on timeout), and the ComputeRequest controller (reconcile loop fires repeatedly). All three need operations that can be called multiple times without side effects.

## Decision

Every mutating operation (create, deploy, hibernate, import) is idempotent:

- **Create:** check if the resource exists first. If it exists and matches the desired state, return success. If it exists with a different state, update it. Only create when absent.
- **Delete:** if the resource doesn't exist, return success (not an error).
- **Update/Patch:** apply the desired state unconditionally — patching an already-correct resource is a no-op.

Implementation pattern for all UC packages:

```go
func (p *Provisioner) CreateCluster(ctx context.Context, name string, opts Options) error {
    existing, err := p.client.Get(ctx, gvr, ns, name)
    if err == nil {
        // Already exists — verify it matches desired state or patch
        return p.reconcile(ctx, existing, opts)
    }
    if !apierrors.IsNotFound(err) {
        return err
    }
    // Does not exist — create
    _, err = p.client.Create(ctx, gvr, ns, build(name, opts))
    return err
}
```

## Consequences

- **Pro:** Safe for the controller reconcile loop — calling CreateCluster on every reconcile is correct
- **Pro:** CLI users can retry failed commands without cleanup
- **Pro:** MCP tool retries are safe — Claude can call the same tool again without fear
- **Pro:** Simplifies error handling — callers don't need to distinguish "already exists" from success
- **Con:** Every mutating operation requires a GET before a CREATE, adding one extra API call
- **Con:** Reconcile logic (what to do when the resource exists but differs) must be defined per UC
