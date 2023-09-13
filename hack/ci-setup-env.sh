#!/usr/bin/env bash
set -ueo pipefail

export KUBECONFIG=./kubeconfig.yaml

terraform init
terraform apply -auto-approve
skaffold run
