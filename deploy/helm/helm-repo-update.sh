
# Modify Chart.yaml based on whether workflow was triggered by a new tag or a file update in ./deploy/helm
echo "GITHUB_REF: ${GITHUB_REF##*/}"
# if [[ ${GITHUB_REF##*/} =~ "v"[0-9].*\.[0-9].*\.[0-9].* ]]; then
# 	sed -i -e "s/^appVersion:.*/appVersion: ${GITHUB_REF##*/}/" ./deploy/helm/kuberhealthy/Chart.yaml
# 	sed -i -e "s/^version:.*/version: $GITHUB_RUN_NUMBER/" ./deploy/helm/kuberhealthy/Chart.yaml
# else
# 	echo "invalid github reference supplied. exiting."
# 	exit 0
# fi

# When a tag is created, we up the appVersion in the chart, but each time we merge to master 
# we will up the chart version to match the github action number
if [[ ${GITHUB_REF##*/} =~ "v"[0-9].*\.[0-9].*\.[0-9].* ]]; then
	sed -i -e "s/^appVersion:.*/appVersion: ${GITHUB_REF##*/}/" ./deploy/helm/kuberhealthy/Chart.yaml
fi
sed -i -e "s/^version:.*/version: $GITHUB_RUN_NUMBER/" ./deploy/helm/kuberhealthy/Chart.yaml


# The github action we use has helm 3 (required) as 'helmv3' in its path, so we alias that in and use it if present
HELM="helm"
if which helmv3; then
    echo "Using helm v3 alias"
    HELM="helmv3"
fi

$HELM version

# Lint helm chart
$HELM lint ./deploy/helm/kuberhealthy
if [ "$?" -ne "0" ]; then
  echo "Linting reports error"
  exit 1
fi

mkdir -p ./helm-repos/tmp.d

# Package Helm Charts
$HELM package --version $GITHUB_RUN_NUMBER -d ./helm-repos/tmp.d ./deploy/helm/kuberhealthy

# Append new package info to top of helm index file
$HELM repo index ./helm-repos/tmp.d --merge ./helm-repos/index.yaml --url https://kuberhealthy.github.io/kuberhealthy/helm-repos/archives

# Move the newly packaged .tgz file and index.yaml where they need to be
mv -f ./helm-repos/tmp.d/kuberhealthy-*.tgz ./helm-repos/archives
mv -f ./helm-repos/tmp.d/index.yaml ./helm-repos
rm -rf ./helm-repos/tmp.d
