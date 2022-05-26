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

# Google container cluster(GKE) configuration

locals {
  pool_count      = length(var.node_pools)
  node_pool_names = [for np in toset(var.node_pools) : np.name]
  node_pools      = zipmap(local.node_pool_names, tolist(toset(var.node_pools)))
}

resource "google_container_cluster" "container_cluster" {
  name                     = var.name
  project                  = var.project_id
  description              = format("%s-gke-cluster", var.name)
  initial_node_count       = 1
  remove_default_node_pool = true
  location                 = var.region
  node_locations           = var.node_locations
  enable_shielded_nodes    = true
  network                  = var.network_link
  subnetwork               = var.subnetwork_link

  master_auth {
    client_certificate_config {
      issue_client_certificate = false
    }
  }

  private_cluster_config {
    enable_private_endpoint = var.enable_private_endpoint
    enable_private_nodes    = var.enable_private_nodes
  }

  resource_labels = {
    environment  = format("%s", var.environment)
    provision_id = format("%s", var.provision_id)
  }

  addons_config {
    http_load_balancing {
      disabled = false
    }
  }

  provisioner "local-exec" {
    command = format("gcloud container clusters get-credentials %s --region %s --project %s",
      google_container_cluster.container_cluster.name,
      google_container_cluster.container_cluster.location, var.project_id
    )
  }
}

resource "google_container_node_pool" "container_node_pool" {
  provider   = google
  project    = var.project_id
  cluster    = google_container_cluster.container_cluster.name
  node_count = var.initial_node_count

  for_each       = local.node_pools
  name           = each.value["name"]
  location       = var.region
  node_locations = [each.value["location"]]

  node_config {
    machine_type = var.machine_type
    preemptible  = true
    tags         = ["http", "ssh"]
    metadata     = {
      disable-legacy-endpoints = "true"
    }
    labels = tomap({ split("=", each.value["label"])[0] : split("=", each.value["label"])[1] })

    service_account = var.service_account
    oauth_scopes    = [
      "https://www.googleapis.com/auth/devstorage.read_write",
      "https://www.googleapis.com/auth/logging.write",
      "https://www.googleapis.com/auth/monitoring",
      "https://www.googleapis.com/auth/compute",
      "https://www.googleapis.com/auth/servicecontrol",
      "https://www.googleapis.com/auth/service.management.readonly",
      "https://www.googleapis.com/auth/trace.append",
      "https://www.googleapis.com/auth/logging.write",
      "https://www.googleapis.com/auth/monitoring",
    ]
  }
}
