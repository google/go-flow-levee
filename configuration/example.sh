echo "Working directory: $(pwd)"

# Get go bin
GB=$(go env GOBIN) GB=${GB:-$(go env GOPATH)/bin}
# go vet dislikes relative pathing.
SCRIPT_DIR=$( cd "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )
# analysis arguments
CONFIG_FILE=${SCRIPT_DIR}/example-config.yaml
PKGS=$(pwd)/...

set -x

go install github.com/google/go-flow-levee/cmd/levee
go vet -vettool ${GB}/levee -config ${CONFIG_FILE} ${PKGS}
