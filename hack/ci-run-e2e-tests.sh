#!/usr/bin/env bash
set -ueo pipefail

export KUBECONFIG=./kubeconfig.yaml

go test ./test/e2e
