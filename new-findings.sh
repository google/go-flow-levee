#!/bin/bash

go_src_path="$(go env GOPATH)/src"
current_branch=$(git branch --show-current)

if [[ ${current_branch} == "master" ]]; then
    echo  "Please run from feature branch"
    exit 1
fi

echo "Switching to master"

git checkout master

echo "Installing levee from master"

cd cmd/levee

go install

echo "Running levee(master) against k8s"

cd $go_src_path/k8s.io/kubernetes

CFG_PATH="$(realpath ./hack/testdata/levee/levee-config.yaml)"

make clean

master_findings=$(mktemp)

make vet WHAT="-vettool=$(which levee) -config=$CFG_PATH" 2> $master_findings

cd $go_src_path/go-flow-levee

echo "Switching to " $current_branch

git checkout $current_branch

echo "Installing levee from" $current_branch

cd cmd/levee

go install

echo "Running levee($current_branch) against k8s"

cd $go_src_path/k8s.io/kubernetes

CFG_PATH="$(realpath ./hack/testdata/levee/levee-config.yaml)"

make clean

branch_findings=$(mktemp)

make vet WHAT="-vettool=$(which levee) -config=$CFG_PATH" 2> $branch_findings

cd $go_src_path/go-flow-levee

master_findings_sorted=$(mktemp)
branch_findings_sorted=$(mktemp)

sort $master_findings > $master_findings_sorted
rm $master_findings

sort $branch_findings > $branch_findings_sorted
rm $branch_findings

diff $master_findings_sorted $branch_findings_sorted