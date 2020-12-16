#!/bin/bash

GO_SRC_PATH="$(go env GOPATH)/src"
CURRENT_BRANCH=$(git branch --show-current)

if [[ ${CURRENT_BRANCH} == "master" ]]; then
    echo  "Please run from feature branch"
    exit 1
fi

echo "Switching to master"

git checkout master

echo "Installing levee from master"

cd cmd/levee

go install

echo "Running levee(master) against k8s"

cd $GO_SRC_PATH/k8s.io/kubernetes

CFG_PATH="$(realpath ./hack/testdata/levee/levee-config.yaml)"

make clean

MASTER_FINDINGS=$(mktemp)

make vet WHAT="-vettool=$(which levee) -config=$CFG_PATH" 2> $MASTER_FINDINGS

cd $GO_SRC_PATH/go-flow-levee

echo "Switching to " $CURRENT_BRANCH

git checkout $CURRENT_BRANCH

echo "Installing levee from" $CURRENT_BRANCH

cd cmd/levee

go install

echo "Running levee($CURRENT_BRANCH) against k8s"

cd $GO_SRC_PATH/k8s.io/kubernetes

CFG_PATH="$(realpath ./hack/testdata/levee/levee-config.yaml)"

make clean

BRANCH_FINDINGS=$(mktemp)

make vet WHAT="-vettool=$(which levee) -config=$CFG_PATH" 2> $BRANCH_FINDINGS

cd $GO_SRC_PATH/go-flow-levee

master_FINDINGS_SORTED=$(mktemp)
BRANCH_FINDINGS_SORTED=$(mktemp)

sort $MASTER_FINDINGS > $master_FINDINGS_SORTED
rm $MASTER_FINDINGS

sort $BRANCH_FINDINGS > $BRANCH_FINDINGS_SORTED
rm $BRANCH_FINDINGS

diff $master_FINDINGS_SORTED $BRANCH_FINDINGS_SORTED