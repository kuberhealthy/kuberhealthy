## Checker pod reaper

This container deletes kuberhealthy checker pods when they are no longer useful. Checker pods are identified by having a label with the key `kh-check-name`.

If the key `kh-check-name` is found on a pod, then it will be deleted when any of the following are true:

- If the checker pod is older than 3 hours and is `Completed`

- If there are more than 5 checker pods with the same check name in the status `Completed` that were created more recently

- If the checker pod is `Failed` and there are more than 5 `Failed` checker pods of the same type which were created more recently

- If the checker pod is `Failed` and was created more than 5 days ago

This container deletes kuberhealthy jobs when they are no longer useful.

A khjob will be deleted when the following is true:

- If the khjob is older than 15 minutes and is `Completed`
