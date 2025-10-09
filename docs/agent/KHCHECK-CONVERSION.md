# KHCHECK Conversion Webhook

The mutating webhook located in `internal/webhook` keeps the cluster backward
compatible with the legacy `comcast.github.io/v1` `KuberhealthyCheck` (historically
called `khcheck`) custom resource. Whenever the Kubernetes API server receives
an `AdmissionReview` for a legacy object, it calls this webhook so the payload
can be rewritten into the modern `kuberhealthy.github.io/v2` `HealthCheck`
shape before the controller stores or processes it.

> The webhook is intentionally idempotent. If it sees a resource that already
> targets the modern API group, it simply approves the request without
> modification.

## Request Flow

1. **AdmissionReview intake** – The API server POSTs a JSON `AdmissionReview`
   object to the `/api/convert` handler. The webhook reads and unmarshals the
   body, logging the resource group, version, kind, namespace, and name for
   traceability.
2. **Early exits** –
   - Delete operations are allowed to pass through untouched so that removing a
     legacy resource never creates a replacement.
   - If the incoming object already belongs to a non-legacy API group, the
     webhook immediately returns `Allowed: true` with no patch.
   - Requests lacking `TypeMeta` information are normalized using the
     `resource` stanza from the `AdmissionRequest` to deduce the kind.
3. **Conversion attempt** – When the resource belongs to the legacy
   `comcast.github.io/v1` group, `convertLegacy` constructs a
   `kuberhealthy.github.io/v2` `HealthCheck` by:
   - Copying the object into the v2 struct and updating `apiVersion`, `kind`,
     and `GroupVersionKind` metadata.
   - Translating the legacy pod layout so embedded `podAnnotations` and
     `podLabels` become the modern `CheckPodMetadata`, and the legacy `podSpec`
     fills the v2 `CheckPodSpec` when the new fields are empty.
   - Returning a human-readable warning that surfaces in the final
     `AdmissionResponse`.

## Persisting Converted Resources

When `ConfigureClient` runs during process startup, it injects three handlers
the webhook can call during conversion:

- **Creation handler** – Creates or updates the converted `HealthCheck` inside
  the v2 API group. Metadata is sanitized (UID, resource version, managed
  fields, etc.) so the server accepts the object as a new resource. If the
  target already exists, its labels, annotations, and spec are updated in place.
- **Legacy deleter** – Issues best-effort deletes for the original
  `comcast.github.io/v1` object. The webhook schedules a background goroutine
  that retries for up to 30 seconds so the legacy object disappears once the v2
  copy is stored.
- **Cleanup scheduler** – By default, calls the deleter on a ticker until the
  object is gone or the timeout expires. Implementations can be swapped during
  tests via `SetLegacyHandlers`.

If no client is configured, the webhook simply allows the legacy request to
pass through unchanged. Conversion only occurs when the controller injects the
creation and deletion handlers during startup.

## AdmissionResponse Details

After creating the v2 representation, the webhook returns an admission response
with `Allowed: true` and a warning describing the conversion. The original
legacy request is written as-is, and the controller's cleanup loop removes it
once the v2 object exists.

## Compatibility Notes

- The webhook recognizes several aliases (`khcheck`, `khchecks`,
  `kuberhealthycheck`, etc.) so manifests using old resource names still
  convert correctly.
- Pods launched by existing controllers continue to be cleaned up because the
  webhook records the conversion in structured logs and schedules removal of the
  original v1 object after the modern copy exists.
- Errors from conversion are translated to `AdmissionResponse.Result.Message`
  values, helping cluster operators diagnose invalid manifests without digging
  through controller logs.
- The `MutatingWebhookConfiguration` now uses `failurePolicy: Fail`, which means
  legacy manifests are rejected outright if the conversion endpoint or its TLS
  assets are unavailable. Rotate the serving certificate with
  `deploy/base/scripts/generateWebhookcert.sh` when needed so the API server
  maintains trust in the webhook service.
- The `kuberhealthy-manager` cluster role must allow deleting
  `khchecks.comcast.github.io` resources; otherwise the cleanup loop logs
  forbidden errors and the legacy objects linger alongside the converted
  `HealthCheck`.

This behavior ensures upgrades from the legacy `comcast.github.io` API group to
`kuberhealthy.github.io/v2` remain seamless while giving cluster operators
clear observability into each conversion event.
