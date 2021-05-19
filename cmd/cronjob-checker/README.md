## CronJob Event Checker

The cronjob-checker fetches a list of all cronjobs in a namespace. After calculating the cron schedule, the check determines the last time the cronjob should have been scheduled through a simulation. It then verifies the last scheduled time was at the simulated time with a tolerance of plus or minus 30 seconds.

#### Example CronJob Event Checker

```yaml
---
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: cronjob-checker
spec:
  runInterval: 5m
  timeout: 10m
  podSpec:
    serviceAccountName: cronjob-checker
    containers:
      - name: cronjob-checker
        image: kuberhealthy/cronjob-checker:v2.1.0
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
kind: RoleBinding
metadata:
  name: cronjob-checker
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: kuberhealthy
subjects:
  - kind: ServiceAccount
    name: cronjob-checker
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: cronjob-checker
```

#### How-to

Apply a `.yaml` file similar to the one shown above with `kubectl apply -f`
