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

variable "name" {
  description = "Name is the prefix to use for resources that needs to be created."
  type        = string
}

variable "environment" {
  description = "Name of the environment where infrastructure being built."
  type        = string
}

variable "provision_id" {
  description = "The ID for the infrastructure provisioning."
  type        = string
  default     = "unknown"
}

variable "project_id" {
  description = "The ID of the project to create the bucket in."
  type        = string
}

variable "region" {
  description = "The region where terraform builds resources."
  type        = string
}

variable "storage_class" {
  description = "Storage class for the gcs bucket"
  type        = string
  default     = null
}

variable "bucket_policy_only" {
  description = "Enables Bucket Policy Only access to a bucket."
  type        = bool
  default     = true
}

variable "role" {
  description = "Role of the google storage bucket iam member"
  type        = string
  default     = "roles/storage.admin"
}

variable "service_account" {
  description = "service account email address"
  type        = string
}
