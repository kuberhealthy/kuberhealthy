## CronJob Event Checker

The cronjob-checker reaches out to and retrieves events from cronjobs in the deployed namespace. It then ranges over the events for the reason FailedNeedsStart indicating it has stopped scheduling. If there is an event with FailedNeedsStart it will alert kuberhealthy.

The check will exit if it is unable to retrieve events from cronjobs.

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
        image: kuberhealthy/cronjob-checker:v1.2.0
        imagePullPolicy: IfNotPresent
        env:
          - name: NAMESPACE
            valueFrom:
              fieldRef:
                fieldPath: metadata.namespace
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
