#!/usr/bin/env bash

# Copyright 2017 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

# Ensure tooling is installed
if [[ -z `which client-gen` ]]; then go install k8s.io/code-generator/cmd/client-gen@latest; fi
if [[ -z `which deepcopy-gen` ]]; then go install k8s.io/code-generator/cmd/deepcopy-gen@latest; fi
#if [[ -z `which register-gen` ]]; then go install k8s.io/code-generator/cmd/register-gen@latest; fi

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
#CODEGEN_PKG=${CODEGEN_PKG:-$(cd "${SCRIPT_ROOT}"; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}

# generate registration
#register-gen --go-header-file="./boilerplate.go.txt"

# generate deepcopy
deepcopy-gen --bounding-dirs="github.com/kuberhealthy/kuberhealthy/v2/pkg/apis/khcheck/v1" \
             --output-file="../pkg/apis/khcheck/v1/zz_generated.deepcopy.go" \
             --go-header-file="./boilerplate.go.txt"

# generate client
source "./kube_codegen.sh"
THIS_PKG="github.com/kuberhealthy/kuberhealthy/v2"

kube::codegen::gen_helpers \
    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
    "${SCRIPT_ROOT}/pkg/apis"

kube::codegen::gen_client \
    --with-watch \
    --output-dir "${SCRIPT_ROOT}/pkg/generated" \
    --output-pkg "${THIS_PKG}/pkg/generated" \
    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
    "${SCRIPT_ROOT}/pkg/apis"
