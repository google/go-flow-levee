#!/bin/bash

set -ex

install_and_run_levee_against_k8s_from_branch() {
    local branch_name=$1
    git checkout "$branch_name"
    cd "${LEVEE_DIR}/cmd/levee" || exit
    go install
    cd "${GO_SRC_PATH}/k8s.io/kubernetes" || exit
    CFG_PATH="$(realpath ./hack/testdata/levee/levee-config.yaml)"
    make clean
    FINDING=$(mktemp)
    set +e
    make vet WHAT="-vettool=$(which levee) -config=$CFG_PATH" 2> "$FINDING"
    set -e
    cd "$LEVEE_DIR"
}

CURRENT_DIR=$(pwd)
LEVEE_DIR=$(dirname $(dirname $(realpath $0)))
GO_SRC_PATH="$(go env GOPATH)/src"
FEATURE_BRANCH=$(git branch --show-current)

if [[ ${FEATURE_BRANCH} == "master" ]]; then
    echo  "Please run from feature branch"
    exit 1
fi

install_and_run_levee_against_k8s_from_branch "master"
MASTER_FINDINGS=$FINDING

install_and_run_levee_against_k8s_from_branch "$FEATURE_BRANCH" 
FEATURE_BRANCH_FINDINGS=$FINDING

FEATURE_BRANCH_FINDINGS_SORTED=$(mktemp)
sort "$FEATURE_BRANCH_FINDINGS" > "$FEATURE_BRANCH_FINDINGS_SORTED"
rm "$FEATURE_BRANCH_FINDINGS"

MASTER_FINDINGS_SORTED=$(mktemp)
sort "$MASTER_FINDINGS" > "$MASTER_FINDINGS_SORTED"
rm "$MASTER_FINDINGS"

diff "$MASTER_FINDINGS_SORTED" "$FEATURE_BRANCH_FINDINGS_SORTED"

cd "$CURRENT_DIR"