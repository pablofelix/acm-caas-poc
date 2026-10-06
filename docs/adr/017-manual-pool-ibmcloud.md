# ADR-017: Manual ClusterPool for IBM Cloud (Hive Pool Limitation Workaround)

## Status

Accepted

## Context

UC-25 requires pre-warmed cluster pools — clusters provisioned ahead of time, hibernated, and made available for instant claiming (~5-10 minutes resume vs ~40 minutes fresh provision).

Hive provides a native ClusterPool CRD for this. However, during PoC testing (2026-09-24), we discovered that Hive's ClusterPool controller only supports five platforms: AWS, GCP, Azure, OpenStack, and VSphere. IBM Cloud is **not** supported.

The CRD schema includes `ibmcloud` as a valid platform field (it shares the `Platform` struct with ClusterDeployment), so the admission webhook accepts the resource. But the controller's `createCloudBuilder()` function in `clusterpool_controller.go` has no `case platform.IBMCloud` — it falls through to `default: "unsupported platform"`. The pool enters `MissingDependencies` and never provisions.

Key findings:

- Confirmed in the Hive source on the `master` branch — this is not a version issue
- No PR or issue exists requesting IBM Cloud pool support
- ClusterDeployment provisioning for IBM Cloud works correctly (UC-01 validated)
- The limitation is in the pool controller's credential propagation, not in the platform's provisioning capability

## Decision

Implement a manual pool orchestration layer in `internal/pool/manual.go` that uses existing UC packages to simulate ClusterPool behaviour for any platform:

1. **Create** — provisions N clusters via `provisioning.Create()` with pool membership labels (`acmlab.redhat.com/pool`, `acmlab.redhat.com/pool-claimed`)
2. **Wait** — watches all pool clusters until installed, then hibernates each via `lifecycle.Hibernate()`
3. **Claim** — finds the first hibernated, unclaimed cluster, resumes it via `lifecycle.Resume()`, marks it claimed
4. **Release** — hibernates the cluster back and marks it unclaimed
5. **Delete** — destroys all clusters in the pool via `provisioning.Destroy()`

The pool manager (`pool.NewWithManagers`) accepts `provisioning.Manager` and `lifecycle.Manager` as dependencies. The existing Hive-based pool operations remain available via `pool.New()` for platforms where ClusterPool is supported.

## Alternatives considered

- **Wait for upstream Hive fix** — no IBM Cloud pool PR exists or is planned. The fix would require implementing `IBMCloudCloudBuilder` in Hive's clusterpool controller, covering credential propagation, install-config generation, and DNS zone delegation. Not viable for PoC timeline.
- **Use AWS for pool testing** — works, but requires a working base domain (the DevShift delegation was broken during testing) and doesn't validate the multi-cloud story.
- **Upgrade MCE/ACM** — Hive is bundled with MCE; upgrading the entire stack for a feature that doesn't exist in any version would not help.

## Consequences

- Manual pools work on any platform that `provisioning.Create()` supports, including IBM Cloud
- No dependency on Hive ClusterPool CRD version or platform support matrix
- Pool membership is tracked via Kubernetes labels on ClusterDeployments, queryable via standard selectors
- No automatic replacement when a cluster is claimed (could be added as a controller or polling loop)
- For CaaS production: the manual pool approach provides more control and full multi-cloud coverage. Hive ClusterPool can remain as an optional backend for AWS/GCP/Azure where it is well-tested. The recommendation is to adopt manual pools as the default, with the Hive backend as a platform-specific optimisation
