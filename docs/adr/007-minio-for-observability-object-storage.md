# ADR-007: MinIO for observability object storage in PoC, cloud object storage in production

## Status

Accepted

## Context

ACM's MultiClusterObservability requires S3-compatible object storage for Thanos. The hub runs on IBM Cloud VPC (ROSA HCP). Options:

1. **MinIO in-cluster** — single pod with a PVC, deployed by acmlab
2. **IBM Cloud Object Storage (COS)** — managed service, requires HMAC credentials and bucket setup
3. **OpenShift Data Foundation (ODF)** — NooBaa/Ceph, heavy operator install

## Decision

**PoC:** Use MinIO deployed as a single pod with a PVC (`ibmc-vpc-block-10iops-tier`, 20Gi). The `acmlab monitor setup` command deploys MinIO, creates the Thanos secret, and creates the MCO CR — all idempotent. `acmlab monitor teardown` removes everything cleanly.

**Production:** Replace MinIO with IBM Cloud Object Storage (COS) or the cloud provider's native object storage. The only change is the Thanos secret content — the MCO CR and all other configuration remain identical.

```yaml
# PoC — MinIO
type: s3
config:
  bucket: thanos
  endpoint: minio.open-cluster-management-observability.svc.cluster.local:9000
  insecure: true
  access_key: minio
  secret_key: minio123

# Production — IBM COS
type: s3
config:
  bucket: acm-observability-prod
  endpoint: s3.us-south.cloud-object-storage.appdomain.cloud
  access_key: <HMAC-access-key>
  secret_key: <HMAC-secret-key>
```

## Consequences

- **Pro:** PoC is self-contained — no external service dependencies
- **Pro:** Setup and teardown are fully automated and idempotent
- **Pro:** Migration to production is a secret change, not an architecture change
- **Pro:** MinIO PVC uses the same storage class as other workloads — no new infra
- **Con:** MinIO is single-replica, not HA — acceptable for PoC, not for production
- **Con:** MinIO PVC data is lost on teardown — acceptable since Thanos metrics are ephemeral for the PoC
- **Con:** MinIO credentials are hardcoded constants — production must use proper secret management
