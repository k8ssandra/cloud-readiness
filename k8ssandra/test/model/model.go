package model

/**
Copyright 2022 DataStax, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

 https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
**/

import (
	"github.com/gruntwork-io/terratest/modules/k8s"
)

type CloudConfig struct {
	Type        string `json:"type,omitempty"`
	Location    string `json:"location,omitempty"`
	Region      string `json:"region,omitempty"`
	Project     string `json:"project,omitempty"`
	Name        string `json:"name,omitempty"`
	CredPath    string `json:"cred_path,omitempty"`
	CredKey     string `json:"cred_key,omitempty"`
	Environment string `json:"environment,omitempty"`
	MachineType string `json:"machine_type,omitempty"`
	Bucket      string `json:"bucket,omitempty"`
}
type TFConfig struct {
	ModuleFolder string `json:"module_folder,omitempty"`
}
type HelmConfig struct {
	ChartPath string `json:"chart_path,omitempty"`
}

type K8cConfig struct {
	Version                 string `json:"version,omitempty"`
	MedusaSecretName        string `json:"medusa_secret_name,omitempty"`
	MedusaSecretFromFileKey string `json:"medusa_secret_from_file_key,omitempty"`
	MedusaSecretFromFile    string `json:"medusa_secret_from_file,omitempty"`
	ValuesFilePath          string `json:"values_file_path,omitempty"`
	ClusterScoped           bool   `json:"cluster_scoped,omitempty"`
}

type ProvisionConfig struct {
	PreTestCleanup     bool        `json:"pre_test_cleanup,omitempty"`
	PostTestCleanup    bool        `json:"post_test_cleanup,omitempty"`
	CleanOnly          bool        `json:"clean_only,omitempty"`
	CleanDir           string      `json:"clean_dir,omitempty"`
	DefaultRetries     int         `json:"default_retries,omitempty"`
	DefaultSleepSecs   int         `json:"default_sleep_secs,omitempty"`
	DefaultTimeoutSecs int         `json:"default_timeout_secs,omitempty"`
	HelmConfig         HelmConfig  `json:"helm_config"`
	TFConfig           TFConfig    `json:"tf_config"`
	CloudConfig        CloudConfig `json:"cloud_config"`
	K8cConfig          K8cConfig   `json:"k_8_c_config"`
}

type ProvisionResult struct {
	Success bool `json:"success,omitempty"`
}

type ContextConfig struct {
	Name          string   `json:"name,omitempty"`
	Namespace     string   `json:"namespace,omitempty"`
	ClusterLabels []string `json:"cluster_labels,omitempty"`
}

type ContextOption struct {
	FullName       string                `json:"full_name,omitempty" yaml:"full_name"`
	KubectlOptions *k8s.KubectlOptions   `json:"kubectl_options" yaml:"kubectl_options"`
	ServiceAccount ContextServiceAccount `json:"service_account" yaml:"service_account"`
	ServerAddress  string                `json:"server_address" json:"server_address"`
}

type ReadinessConfig struct {
	ProvisionConfig          ProvisionConfig          `json:"provision_config"`
	KubectlConfigPath        string                   `json:"kubectl_config_path,omitempty"`
	UniqueId                 string                   `json:"unique_id,omitempty"`
	RootFolder               string                   `json:"root_folder,omitempty"`
	Contexts                 map[string]ContextConfig `json:"contexts,omitempty"`
	ServiceAccountNamePrefix string                   `json:"service_account_name_prefix,omitempty"`
	ExpectedNodeCount        int                      `json:"expected_node_count,omitempty"`
}

type ContextServiceAccount struct {
	Name      string `json:"name" yaml:"name"`
	Secret    string `json:"secret" yaml:"secret"`
	Token     string `json:"token" yaml:"token"`
	Cert      string `json:"cert" yaml:"cert"`
	Namespace string `json:"namespace" yaml:"namespace"`
}

type ProvisionMeta struct {
	ProvisionId    string            `json:"provision_id,omitempty"`
	KubeConfigs    map[string]string `json:"kube_configs,omitempty"`
	ServiceAccount string            `json:"service_account"`
}