## CronJob Event Checker

The cronjob-checker reaches out to a specified namespace and retrieves events from cronjobs. It then ranges over the events for a specified reason. If there is an event with the specified reason it will alert kuberhealthy.

You can specify the namespace to check with the `NAMESPACE` environment variable in the `.yaml` file.

You can specify the event reason to check with the `REASON` environment variable in the `.yaml` file.

The check will exit if it is unable to retrieve events from the cronjob in a given namespace.

namespace fields will need to be updated to applied namespace to give permissions appropriately to the check.

#### Example CronJob Event Checker

```yaml
---
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: cronjob-checker
  namespace: kuberhealthy
spec:
  runInterval: 1h
  timeout: 10m
  podSpec:
    serviceAccountName: cronjob-checker
    containers:
      - name: cronjob-checker
        image: kuberhealthy/cronjob-checker:v1.1.0
        imagePullPolicy: IfNotPresent
        env:
          - name: NAMESPACE
            value: "kuberhealthy"
          - name: REASON
            value: "FailedNeedsStart"
        resources:
          requests:
            cpu: 15m
            memory: 15Mi
          limits:
            cpu: 25m
        restartPolicy: Always
    terminationGracePeriodSeconds: 5
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: cronjob-checker
rules:
  - apiGroups:
      - ""
      - "events.k8s.io"
    resources:
      - events
    verbs:
      - get
      - list
      - watch
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: cronjob-checker
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cronjob-checker
subjects:
  - kind: ServiceAccount
    name: cronjob-checker
    namespace: kuberhealthy
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: cronjob-checker
  namespace: kuberhealthy
```

#### How-to

Make sure you are using the latest release of Kuberhealthy 2.0.0.

Apply a `.yaml` file similar to the one shown above with `kubectl apply -f`
