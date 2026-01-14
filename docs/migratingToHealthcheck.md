# Migrating to `HealthCheck`

`HealthCheck` replaces the legacy `healthcheck` schema. The v3 API uses a dedicated CRD with explicit status fields and stricter validation.

## Checklist

1. Install v3 (clean reinstall).
2. Recreate checks using the v3 `HealthCheck` schema.
3. Update any automation that parses the old status output.

Use the examples in [CHECKS_REGISTRY.md](CHECKS_REGISTRY.md) as a starting point.
