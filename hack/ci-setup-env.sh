#!/usr/bin/env bash
set -ueo pipefail

export KUBECONFIG=$(pwd)/kubeconfig.yaml

terraform init
terraform apply -var "hcloud_token=$HCLOUD_TOKEN" -auto-approve
skaffold run
