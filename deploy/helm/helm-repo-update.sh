#!/bin/bash
# the github action we use has helm 3 (required) as 'helmv3' in its path, so we alias that in and use it if present
HELM="helm"
if which helmv3; then
    echo "Using helm v3 alias"
    HELM="helmv3"
fi

$HELM version

$HELM lint ./kuberhealthy
if [ "$?" -ne "0" ]; then
  echo "Linting reports error"
  exit 1
fi
$HELM package --app-version ${GITHUB_REF##*/} --version $GITHUB_RUN_ID -d ../../helm-repos/archives ./kuberhealthy
#HELM package --app-version ${GITHUB_REF##*/} --version ${GITHUB_REF##*/} -d ../../helm-repos/archives ./kuberhealthy
#cd ../../helm-repos/archives
cd ../../helm-repos
$HELM repo index ./archives --merge ./index.yaml --url https://comcast.github.io/kuberhealthy/helm-repos/archives
mv -f ./archives/index.yaml ./index.yaml
