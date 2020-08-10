## Kuberhealthy Release Process

To release a new version of Kuberhealthy:

1. Verify that the master branch has all the latest changes

2. Tag the new release image tag and build the image
    - To tag and build the new release:
    - Update the `TAG` in the [Makefile](../cmd/kuberhealthy/Makefile)
    - In the Kuberhealthy project directory, run `make build` to build the image
    
3. Test the new image locally:
    - Install Kuberhealthy locally running the installation command found [here](../README.md#installation)
    - Make sure to supply your own [values.yaml](../deploy/helm/kuberhealthy/values.yaml), updating the `image.tag` to the new release imag tag and and `deployment.imagePullPolicy` to `Never`
    - Wait for all the kuberhealthy checks to come up and run as expected
    
4. Push image to [Dockerhub](https://hub.docker.com/r/kuberhealthy/kuberhealthy)
    - In the Kuberhealthy project directory, run `make push` to push the image

5. Soak test in lab and prod for several days

6. Create a PR updating the project with the latest Kuberhealthy tag:
    - [Makefile](../cmd/kuberhealthy/Makefile)
    - Helm [values.yaml](../deploy/helm/kuberhealthy/values.yaml)
    - Integration test values [values.yaml](../.ci/values.yaml)
    - Once the PR is merged, make sure the Flat File Generation PR is also merged -- this updates the deployment flat spec files with the latest Kuberhealthy release image tag

7. Write up and publish release notes, giving an update / latest changes from the last release: new *Kuberhealthy Checks* (if any), new *Features*, and *Bug Fixes*
    - Tag the issue and pr as well as the contributor to the release notes
 
8. When the release is published, the new version tag is automatically created. This new tag triggers the following github actions:
    - PR is created to update the Helm repo with the new version tag. Merge this PR. 
    - Once this PR for the Helm repo is merged, the gh-pages branch is updated with the latest version of master

Release done!