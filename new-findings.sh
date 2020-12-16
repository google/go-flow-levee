#!/bin/bash

set -x

install_and_run_levee_against_k8s_from_branch() {
    branch_name=$1
    echo "Switching to ${branch_name}"
    git checkout "$branch_name"
    echo "Installing levee from ${branch_name}"
    cd cmd/levee || exit
    go install
    echo "Running levee(master) against k8s"
    cd "${GO_SRC_PATH}/k8s.io/kubernetes" || exit
    CFG_PATH="$(realpath ./hack/testdata/levee/levee-config.yaml)"
    make clean
    findings=$(mktemp)
    make vet WHAT="-vettool=$(which levee) -config=$CFG_PATH" 2> "$findings"
    cd "${GO_SRC_PATH}/go-flow-levee" || exit
    return "$findings"
}

GO_SRC_PATH="$(go env GOPATH)/src"
FEATURE_BRANCH=$(git branch --show-current)

if [[ ${FEATURE_BRANCH} == "master" ]]; then
    echo  "Please run from feature branch"
    exit 1
fi

MASTER_FINDINGS=install_and_run_levee_against_k8s_from_branch "master"

FEATURE_BRANCH_FINDINGS=install_and_run_levee_against_k8s_from_branch $FEATURE_BRANCH

diff <(sort "$MASTER_FINDINGS_SORTED") <(sort "$BRANCH_FINDINGS_SORTED")
