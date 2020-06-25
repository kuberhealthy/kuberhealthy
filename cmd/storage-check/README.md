## Storage Check

This check tests if a persistent volume claim (`PVC`) can be created and used within your Kubernetes cluster. It will attempt to create a `PVC` using either the default storage class (`SC`) or a user specified one. When the `PVC` is successfully created, the check will initialize the storage with a known value and then discover the nodes in the cluster and attempt to use that `PVC` on each node. If the check can create a `PVC`, initialize the `PVC` and use/mount and verify the contents of the storage on each discovered (or explicitly allowed/ignored) node then the check will succeed and you can have confidence in your ability to mount storage on nodes that are allowed to be scheduled. 

Once the contents of the `PVC` have been validated, the check `Job`, the init `Job` and the `PVC` will be cleaned up and the check will be marked as successful.

Container resource requests are set to `15 millicores` of CPU and `20Mi` units of memory and use the Alpine image `alpine:3.11` for the `Job` and a default of `1Gi` for the `PVC`.  If the environment variable `CHECK_STORAGE_PVC_SIZE` is set then the value of that will be used instead of the default.

By default, the nodes of the cluster will be discovered and only those nodes that are `untainted`, in a `Ready` state and not in the role of `master` will be used. If node(s) need to be `ignored` for whatever reason, then the environment variable `CHECK_STORAGE_IGNORED_CHECK_NODES` should be used a space or comma separated list of nodes should be supplied. If `auto-discovery` is not desired, the environment variable `CHECK_STORAGE_ALLOWED_CHECK_NODES` can be used and a space or comma separated list of nodes that should be checked needs to be supplied. If `CHECK_STORAGE_ALLOWED_CHECK_NODES` is supplied and a node in that list matches a node in the ignored (`CHECK_STORAGE_IGNORED_CHECK_NODES`) list then that node will be ignored.

By default, the storage check `Job` and initialize storage check `Job` will use Alpine's `alpine:3.11` image. If a different image is desired, use the environment variable `CHECK_STORAGE_IMAGE` or `CHECK_STORAGE_INIT_IMAGE` depending on which image should be changed.

Initializing the storage is pretty simple and a file with the contents of `storage-check-ok` is created as /data/index.html. There is no reason it's called index.html except maybe for future additional checks. If the storage initialization should be done differently, or needs to be more complex, the option to use a completely different image exists as described above (`CHECK_STORAGE_INIT_IMAGE`). To override the arguments used to create the known data, use the environment variable `CHECK_STORAGE_INIT_COMMAND_ARGS` and change it from the default of `echo storage-check-ok > /data/index.html && ls -la /data && cat /data/index.html`.

Checking the storage is also pretty simple (there is a theme here). The check simply mounts the `PVC` at `/data`, cats the `/data/index.html` file and pipes the output to `grep` looking for the contents of `storage-check-ok`. If it sees that, the `exit code` will be `0` and the check passes for that particular node. Because the `Pod` on the `node` could mount the storage, see the previously created file AND see the known contents of the file the check is `OK`. If a more advanced check is desired, the entire imaged can be changed with the environment variable `CHECK_STORAGE_IMAGE` or to just change the command line arguments use `CHECK_STORAGE_COMMAND_ARGS` and change from the default of `ls -la /data && cat /data/index.html && cat /data/index.html | grep storage-check-ok`.

Custom images can be used for this check and can be specified with the `CHECK_STORAGE_IMAGE` and `CHECK_STORAGE_INIT_IMAGE` environment variables as described above. If a custom image requires the use of environment variables, they can be passed down into the custom container by setting the environment variable `ADDITIONAL_ENV_VARS` to a string of comma-separated values (`"X=foo,Y=bar"`).

A successful run implies that a `PVC` was successfully created and Bound, a `Storage init Job` was able to use the `PVC` and correctly initialize it with known data, and all schedulable nodes were able to run the check `Job` with the mounted `PVC` and validate the contents. A failure implies that an error or timeout occurred anywhere in the `PVC` request, `Init Job` creation, `Check Job` creation, validation of known data, or tear down process -- resulting in an error report to the _Kuberhealthy_ status page.

#### Storage Check Diagram
COMING SOON THIS IS NOT THE STORAGE CHECK
![](../../images/kh-deployment-check.gif)

#### Check Steps

This check follows the list of actions in order during the run of the check:
1.  Looks for old storage check job, storage init job, and PVC belonging to this check and cleans them up.
2.  Creates a PVC in the namespace and waits for the PVC to be ready.
3.  Creates a storage init configuration, applies it to the namespace, and waits for the storage init job to come up and initialize the PVC with known data.
4.  Determine which nodes in the cluster are going to run the storage check by auto-discovery or a list supplied nodes via the `CHECK_STORAGE_IGNORED_CHECK_NODES` and `CHECK_STORAGE_ALLOWED_CHECK_NODES` environment variables.
5.  For each node that needs a check, creates a storage check configuration, applies it to the namespace, and waits for the storage check job to start and validate the contents of storage on each desired node.
6.  Tear everything down once completed.

#### Check Details

- Namespace: kuberhealthy
- Timeout: 15 minutes
- Check Interval: 30 minutes
- Check name: `storage-check`
- Configurable check environment variables:
  - `CHECK_STORAGE_IMAGE`: Storage check container image. (default=`alpine:1.3`)
  - `CHECK_STORAGE_INIT_IMAGE`: Storage initialization container image. (default=`alpine:1.3`)
  - `CHECK_NAMESPACE`: Namespace for the check (default=`kuberhealthy`).
  - `CHECK_STORAGE_ALLOWED_CHECK_NODES`: The explicit list of nodes to check (default=auto-discover worker nodes)
  - `CHECK_STORAGE_IGNORED_CHECK_NODES`: The list of nodes to ignore for the check (default=empty)
  - `CHECK_STORAGE_INIT_COMMAND_ARGS`: The arguments to the storage check data initialization (default=`echo storage-check-ok > /data/index.html && ls -la /data && cat /data/index.html`)
  - `CHECK_STORAGE_COMMAND_ARGS`: The arguments to the storage check command (default=`ls -la /data && cat /data/index.html && cat /data/index.html | grep storage-check-ok`)
  - `CHECK_POD_CPU_REQUEST`: Check pod deployment CPU request value. Calculated in decimal SI units `(15 = 15m cpu)`.
  - `CHECK_POD_CPU_LIMIT`: Check pod deployment CPU limit value. Calculated in decimal SI units `(75 = 75m cpu)`.
  - `CHECK_POD_MEM_REQUEST`: Check pod deployment memory request value. Calculated in binary SI units `(20 * 1024^2 = 20Mi memory)`.
  - `CHECK_POD_MEM_LIMIT`: Check pod deployment memory limit value. Calculated in binary SI units `(75 * 1024^2 = 75Mi memory)`.
  - `ADDITIONAL_ENV_VARS`: Comma separated list of `key=value` variables passed into the pod's containers.
  - `SHUTDOWN_GRACE_PERIOD`: Amount of time in seconds the shutdown will allow itself to clean up after an interrupt signal (default=`30s`).
  - `DEBUG`: Verbose debug logging.

#### Example KuberhealthyStorageCheck Spec

The following configuration will create a storage check for all non-master nodes except node4 using the VMware vsan-default storage class:

```yaml
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: storage-check
  namespace: kuberhealthy
spec:
  runInterval: 5m
  timeout: 10m
  podSpec:
    containers:
    - env: 
        - name: CHECK_STORAGE_NAME
          value: "mysuperfuntime-pv-claim"
        - name: CHECK_STORAGE_PVC_STORAGE_CLASS_NAME
          value: "vsan-default"
        - name : CHECK_STORAGE_IGNORED_CHECK_NODES
          value: "node4"
      image: chrishirsch/kuberhealthy-storage-check:v0.0.1
      imagePullPolicy: IfNotPresent
      name: main
      resources:
        requests:
          cpu: 10m
          memory: 50Mi
      securityContext:
        allowPrivilegeEscalation: false
        readOnlyRootFilesystem: true
    restartPolicy: Never
    serviceAccountName: storage-sa
    securityContext:
      runAsUser: 999
      fsGroup: 999
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: storage-sa
  namespace: kuberhealthy
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: storage-role
  namespace: kuberhealthy
rules:
  - apiGroups:
      - ""
    resources:
      - services
      - persistentvolumeclaims
    verbs:
      - create
      - delete
      - get
      - list
      - patch
      - update
      - watch
  - apiGroups:
      - "batch"
      - "extensions"
    resources:
      - jobs
    verbs:
      - create
      - delete
      - get
      - list
      - patch
      - update
      - watch
  - apiGroups:
      - ""
    resources:
      - pods
    verbs:
      - get
      - list
      - watch
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kuberhealthy-storage-cr
rules:
  - apiGroups:
      - ""
    resources:
      - nodes
    verbs:
      - get
      - list
      - watch
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kuberhealthy-storage-crb
roleRef:
    apiGroup: rbac.authorization.k8s.io
    kind: ClusterRole
    name: kuberhealthy-storage-cr
subjects:
  - kind: ServiceAccount
    name: storage-sa
    namespace: kuberhealthy
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: storage-rb
  namespace: kuberhealthy
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: storage-role
subjects:
  - kind: ServiceAccount
    name: storage-sa

```

#### Install

To use the *Storage Check* with Kuberhealthy, apply the configuration file [storage-check.yaml](storage-check.yaml) to your Kubernetes Cluster. The following command will also apply the configuration file to your current context:

`kubectl apply -f https://raw.githubusercontent.com/Comcast/kuberhealthy/2.0.0/cmd/storage-check/storage-check.yaml`

Make sure you are using the latest release of Kuberhealthy 2.0.0 or later.

The check configuration file contains:
- KuberhealthyCheck
- Role
- Rolebinding
- ClusterRole
- ClusterRoleBinding
- ServiceAccount

The role, rolebinding, clusterrole, clusterrolebinding and service account are all required to create and delete all PVCs and jobs from the check in the given namespaces you install the check for. The assumed default service account does not provide enough permissions for this check to run.
