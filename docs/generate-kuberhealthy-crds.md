## Generating Kuberhealthy CRDs 
- khstate
- khcheck
- khjob

#### Run script:

From the Kuberhealthy directory run:
```bash
./scripts/generate.sh
```

The exit status 1 error is expected and the CRDS are generated properly despite it. 

This should generate the three CRD files in the [scripts/generated](../scripts/generated)
- comcast.github.io_khchecks.yaml
- comcast.github.io_khjobs.yaml
- comcast.github.io_khstates.yaml
