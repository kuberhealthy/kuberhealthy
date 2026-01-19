# Compatibility

This document describes the baseline compatibility and requirements for Kuberhealthy v3.

## Kubernetes version

Kuberhealthy requires Kubernetes versions that support:

- `apiextensions.k8s.io/v1` CustomResourceDefinitions (Kubernetes 1.16+).
- `coordination.k8s.io/v1` Lease objects when leader election is enabled.

If you need support for older Kubernetes versions, use a compatible Kuberhealthy release and verify the CRD API versions in the manifests.

## API versions

Kuberhealthy v3 uses `kuberhealthy.github.io/v2` for the `HealthCheck` CRD and resources. This is the current API version for the v3 controller.
