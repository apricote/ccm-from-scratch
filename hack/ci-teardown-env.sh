#!/usr/bin/env bash
set -ueo pipefail

terraform destroy -var "hcloud_token=$HCLOUD_TOKEN" -auto-approve
