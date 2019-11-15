When doing a relase involving our `yaml` files, we need to keep all the following in sync:

- the helm chart here
- stand-alone yaml files here
- the public helm chart in `helm/charts/stable`


The rollout procedure for yaml changes in our default installation is:

- checkout a new branch `git checkout -b`
- make the change in the helm chart here under `helm/kuberhealthy`
- have Helm 3 installed and run the following script to re-generate the static spec files: `deploy/helm/create-flat-files.sh`
- push the new branch and create a pull request to the `master` branch
- after the pull request is approved to `master`, the public helm chart will need updated
