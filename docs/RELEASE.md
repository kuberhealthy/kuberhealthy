## Kuberhealthy Release Process

To release a new version of Kuberhealthy:

1. Ensure that all issues in the release milestone are completed
1. Verify that the master branch has all the latest changes that have been tested and verified
1. After any running `Build and Push kuberhealthy Latest` action is completed, test the image `kuberhealthy/kuberhealthy:latest` to validate the release
1. If the release looks good, tag it with the new upcoming version and push it to docker hub:
    ```
    docker pull kuberhealthy/kuberhealthy:latest
    docker tag kuberhealthy/kuberhealthy:latest kuberhealthy/kuberhealthy:v[version]
    docker push kuberhealthy/kuberhealthy:v[version]
    ```
1. Create a PR updating the project with the latest Kuberhealthy tag:
    - [Makefile](../cmd/kuberhealthy/Makefile)
    - Helm [values.yaml](../deploy/helm/kuberhealthy/values.yaml)
1. Once the PR is merged, make sure the automatic `Build Flat Spec Files from Helm Chart` PR is also merged
1. Create a `Release` draft on GitHub named with the release version (such as `v2.3.4`)
1. Write up and publish release notes, giving an update / latest changes from the last release: new *Kuberhealthy Checks* (if any), new *Features*, and *Bug Fixes*
    1. Tag the issue and pr as well as the contributor to the release notes
1. Close the GitHub milestone and create a new milestone version for the next release
1 Celebrate!
