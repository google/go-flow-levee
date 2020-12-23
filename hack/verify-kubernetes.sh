#!/bin/bash

set -ex

install_and_run_levee_against_k8s_from_branch() {
    local branch_name=$1
    git checkout "$branch_name"
    local levee_output
    levee_output="$(mktemp -d)/levee"
    go build -o "${levee_output}" "${LEVEE_DIR}/cmd/levee"
    local cfg_path
    cfg_path="$(realpath "${K8S_SRC_PATH}"/hack/testdata/levee/levee-config.yaml)"
    make -C "${K8S_SRC_PATH}" clean
    FINDINGS=$(mktemp)
    set +e
    make -C "${K8S_SRC_PATH}" vet WHAT="-vettool=$levee_output -config=$cfg_path" 2>"${FINDINGS}"
    set -e
}

LEVEE_DIR="$(dirname "$(dirname "$(realpath "$0")")")"
K8S_SRC_PATH="$(go env GOPATH)/src/k8s.io/kubernetes"
FEATURE_BRANCH=$(git branch --show-current)

if [[ "${FEATURE_BRANCH}" == "master" ]]; then
    echo  "Please run from feature branch"
    exit 1
fi

install_and_run_levee_against_k8s_from_branch "master"
MASTER_FINDINGS=$FINDINGS

install_and_run_levee_against_k8s_from_branch "$FEATURE_BRANCH" 
FEATURE_BRANCH_FINDINGS=$FINDINGS

diff <(sort "${FEATURE_BRANCH_FINDINGS}") <(sort "${MASTER_FINDINGS}")
