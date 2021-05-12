## Kuberhealthy Chart Archives


#### Releasing New Versions

- Use `helm package` to make a new helm archive file.  You must be in the `deploy/helm/kuberhealthy/` directory so that the chart is found.
- Add a new section to the `index.yaml` file in this directory and edit the entries to point to the package file created in the prior step

#### Installation

Install the newest Helm chart using the following commands:

1. Create namespace "kuberhealthy" in the desired Kubernetes cluster/context:  
	`kubectl create namespace kuberhealthy`
2. Set your current namespace to "kuberhealthy":  
	`kubectl config set-context --current --namespace=kuberhealthy`
3. Add the kuberhealthy repo to Helm:  
	`helm repo add kuberhealthy https://kuberhealthy.github.io/kuberhealthy/helm-repos`
4. Install kuberhealthy:  
	`helm install kuberhealthy kuberhealthy/kuberhealthy`


