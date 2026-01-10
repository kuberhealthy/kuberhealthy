# KHCHECK Conversion

Kuberhealthy v3 does not include a conversion webhook or any automated migration
path from legacy `KuberhealthyCheck` or `KuberhealthyJob` resources. Legacy
objects must be deleted and replaced with `HealthCheck` resources that follow
the v3 schema.
