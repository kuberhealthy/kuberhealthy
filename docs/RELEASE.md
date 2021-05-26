## Kuberhealthy Release Process

To release a new version of Kuberhealthy:

1. Ensure that all issues in the release milestone are completed
1. Verify that the master branch has all the latest changes that have been tested and verified
1. After any running `Build and Push kuberhealthy Unstable` action is completed, tag a release candidate from the build done in master as unstable
    ```
    docker pull kuberhealthy/kuberhealthy:unstable
    docker tag kuberhealthy/kuberhealthy:unstable kuberhealthy/kuberhealthy:v[version]-rc.1
    docker push kuberhealthy/kuberhealthy:v[version]-rc.1
    ```
1. If the release looks good, tag it with the final new upcoming version along with the `latest` tag and push it to docker hub:
    ```
    docker pull kuberhealthy/kuberhealthy:v[version]-rc.1
    docker tag kuberhealthy/kuberhealthy:v[version]-rc.1 kuberhealthy/kuberhealthy:v[version]
    docker push kuberhealthy/kuberhealthy:v[version]
    docker tag kuberhealthy/kuberhealthy:v[version] kuberhealthy/kuberhealthy:latest
    docker push kuberhealthy/kuberhealthy:latest
    ```
1. Create a `Release` draft on GitHub named with the release version (such as `v2.3.4`)
1. Write up and publish release notes, giving an update / latest changes from the last release: new *Kuberhealthy Checks* (if any), new *Features*, and *Bug Fixes*
    1. Tag the issue and pr as well as the contributor to the release notes
1. Merge auto-generated `Flat Spec File Regeneration ` and `Package and Index for Helm Repo` PRs
1. Close the GitHub milestone and create a new milestone version for the next release
1 Celebrate!
