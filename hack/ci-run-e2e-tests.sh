#!/usr/bin/env bash
set -ueo pipefail

export KUBECONFIG=$(pwd)/kubeconfig.yaml

go test ./test/e2e
