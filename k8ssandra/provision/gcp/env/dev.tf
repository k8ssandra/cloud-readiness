# Copyright 2022 DataStax, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

module "vpc" {
  source       = "../modules/vpc"
  name         = local.prefix
  environment  = var.environment
  region       = var.region
  project_id   = var.project_id
  cidr_block = var.cidr_block
  secondary_cidr_block = var.secondary_cidr_block
}

module "gke" {
  source             = "../modules/gke"
  environment        = var.environment
  provision_id       = var.provision_id
  name               = local.prefix
  region             = var.region
  node_locations     = var.node_locations
  project_id         = var.project_id
  initial_node_count = var.initial_node_count
  machine_type       = var.machine_type
  network_link       = module.vpc.network_selflink
  subnetwork_link    = module.vpc.subnetwork_selflink
  service_account    = module.iam.service_account
  master_ipv4_cidr_block = var.master_ipv4_cidr_block
}

module "iam" {
  source                           = "../modules/iam"
  name                             = local.prefix
  region                           = var.region
  project_id                       = var.project_id
  service_account_custom_iam_roles = var.service_account_custom_iam_roles
  service_account_iam_roles        = var.service_account_iam_roles
  project_services                 = var.project_services
}

module "gcs" {
  source          = "../modules/gcs"
  name            = format("%s-storage-bucket", local.prefix)
  environment     = var.environment
  region          = var.region
  project_id      = var.project_id
  service_account = module.iam.service_account
}
