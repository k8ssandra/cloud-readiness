#!/bin/sh
cd /tmp/%1/k8ssandra/provision/gcp/env || exit
tf plan -destroy -out=destroy-plan
tf apply destroy-plan


