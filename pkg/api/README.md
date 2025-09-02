# api

This package defines the Kubernetes CustomResource definitions used by Kuberhealthy. It exposes the `KuberhealthyCheck` type, its spec and status, and registration helpers for adding these resources to a controller-runtime scheme.

The scope of this package is limited to type definitions and related helpers. Logic for running checks or interacting with the Kubernetes API is delegated to other packages.
