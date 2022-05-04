#!/bin/sh

target=$1
echo "invoked delegate with $1"
cd /tmp/$target/k8ssandra/provision/gcp/env || exit
terraform init
terraform plan -destroy -out=destroy-plan-$target
terraform apply destroy-plan-$target
