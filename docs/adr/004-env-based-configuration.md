# ADR-004: Environment-based configuration with .env

## Status

Accepted

## Context

The lab needs cloud credentials (IBM Cloud API key), cluster connection details (kubeconfig, context), and operational defaults (timeouts, instance types). These values vary per environment and must not be committed to git.

## Decision

All configuration is loaded from environment variables via `internal/config/LoadFromEnv()`. A `.env` file (gitignored) provides local defaults, loaded at startup by `github.com/joho/godotenv`. A `.env.example` is committed as a template.

Non-sensitive values (region, timeout, image set) can also be overridden via CLI flags. Credentials (API keys, kubeconfig paths) are env-only — no CLI flags for secrets.

## Consequences

- **Pro:** No credentials in code or git history
- **Pro:** Same binary works across environments by changing `.env`
- **Pro:** CI/CD can inject env vars directly — no config file management
- **Pro:** `.env.example` documents all required/optional variables in one place
- **Con:** Must remember to copy `.env.example` to `.env` on first setup
- **Con:** No validation until runtime — a typo in a variable name silently uses the default
