#!/bin/bash
# the github action we use has helm 3 (required) as 'helmv3' in its path, so we alias that in and use it if present
HELM="helm"
if which helmv3; then
    echo "Using helm v3 alias"
    HELM="helmv3"
fi

$HELM version
echo "Creating flat kuberhealthy.yaml"
$HELM template --namespace kuberhealthy kuberhealthy > ../kuberhealthy.yaml
if [[ $? -ne 0 ]]; then
	echo "Failed to create flat kuberhealthy.yaml"
	exit 1
fi


# temp helm fix described in issue #279:

cp -f ../kuberhealthy.yaml kuberhealthy.yaml.old
cat kuberhealthy/crds/* > ../kuberhealthy.yaml
cat kuberhealthy.yaml.old >> ../kuberhealthy.yaml