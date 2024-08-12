#!/usr/bin/env bash

# This script is based off the kube-builder script with the name update-codegen.sh,
# but it also includes installation of tools and more build steps

set -o errexit
set -o nounset
set -o pipefail

# Ensure tooling is installed
if [[ -z `which client-gen` ]]; then 
    echo 'go install k8s.io/code-generator/cmd/client-gen@latest'
    go install k8s.io/code-generator/cmd/client-gen@latest
fi
if [[ -z `which deepcopy-gen` ]]; then 
    echo 'go install k8s.io/code-generator/cmd/deepcopy-gen@latest'
    go install k8s.io/code-generator/cmd/deepcopy-gen@latest
fi
if [[ -z `which register-gen` ]]; then 
    echo 'go install k8s.io/code-generator/cmd/register-gen@latest'
    go install k8s.io/code-generator/cmd/register-gen@latest 
fi

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
THIS_PKG="github.com/kuberhealthy/kuberhealthy/v2"
#CODEGEN_PKG=${CODEGEN_PKG:-$(cd "${SCRIPT_ROOT}"; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}

# include codegen funcs from kubernetes project
source "./kube_codegen.sh"

# generate registration
#register-gen --go-header-file="./boilerplate.go.txt"

# generate deepcopy
for CRD in comcast.github.io; do
    echo "Generating CRD go files for $CRD..."
    deepcopy-gen --bounding-dirs="github.com/kuberhealthy/kuberhealthy/v2/pkg/apis/$CRD/v1" \
                --output-file="../pkg/apis/$CRD/v1/zz_generated.deepcopy.go" \
                --go-header-file="./boilerplate.go.txt"
    register-gen ../pkg/apis/$CRD/v1 
done

kube::codegen::gen_helpers \
    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
    "${SCRIPT_ROOT}/pkg/apis"

kube::codegen::gen_client \
    --with-watch \
    --output-dir "${SCRIPT_ROOT}/pkg/generated" \
    --output-pkg "${THIS_PKG}/pkg/generated" \
    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
    "${SCRIPT_ROOT}/pkg/apis"
