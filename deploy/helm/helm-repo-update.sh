#!/bin/bash

#modify Chart.yaml based on whether workflow was triggered by a new tag or a file update in ./deploy/helm
if [[ ${GITHUB_REF##*/} == "v"* ]]; then
	sed -i -e "s/^appVersion:.*/appVersion: ${GITHUB_REF##*/}/" ./resultsFiles/envOutput.yaml
	sed -i -e "s/^version:.*/version: $GITHUB_RUN_NUMBER/" ./resultsFiles/envOutput.yaml
else
	sed -i -e "s/^version:.*/version: $GITHUB_RUN_NUMBER/" ./resultsFiles/envOutput.yaml
fi

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

$HELM package --app-version ${GITHUB_REF##*/} --version $GITHUB_RUN_NUMBER -d ../../helm-repos/tmp.d ./kuberhealthy

cd ../../helm-repos

$HELM repo index ./tmp.d --merge ./index.yaml --url https://comcast.github.io/kuberhealthy/helm-repos/archives

mv -f ./tmp.d/kuberhealthy-$GITHUB_RUN_NUMBER.tgz ./archives
mv -f ./tmp.d/index.yaml ./
