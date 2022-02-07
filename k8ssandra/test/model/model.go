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
	corev1 "k8s.io/api/core/v1"
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
	K8cConfig          K8cConfig   `json:"k8c_config"`
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
	ShortName      string                `json:"short_name" yaml:"short_name"`
	FullName       string                `json:"full_name,omitempty" yaml:"full_name,omitempty"`
	KubectlOptions *k8s.KubectlOptions   `json:"kubectl_options,omitempty"`
	ServiceAccount ContextServiceAccount `json:"service_account" yaml:"service_account,omitempty"`
	ServerAddress  string                `json:"server_address" yaml:"server_address,omitempty"`
	ProvisionMeta  ProvisionMeta         `json:"provision_meta" yaml:"provision_meta"`
}

type ReadinessConfig struct {
	ProvisionConfig          ProvisionConfig          `json:"provision_config"`
	KubectlConfigPath        string                   `json:"kubectl_config_path,omitempty"`
	UniqueId                 string                   `json:"unique_id,omitempty"`
	RootFolder               string                   `json:"root_folder,omitempty"`
	Contexts                 map[string]ContextConfig `json:"contexts,omitempty"`
	ServiceAccountNameSuffix string                   `json:"service_account_name_suffix,omitempty"`
	ExpectedNodeCount        int                      `json:"expected_node_count,omitempty"`
}

type ContextServiceAccount struct {
	Name      string `json:"name" yaml:"name,omitempty"`
	Secret    string `json:"secret" yaml:"secret,omitempty"`
	Token     string `json:"token" yaml:"token,omitempty"`
	Cert      string `json:"cert" yaml:"cert,omitempty"`
	Namespace string `json:"namespace" yaml:"namespace,omitempty"`
}

type ProvisionMeta struct {
	Enabled           bool              `json:"enabled,omitempty"`
	ProvisionId       string            `json:"provision_id,omitempty"`
	KubeConfigs       map[string]string `json:"kube_configs,omitempty"`
	ServiceAccount    string            `json:"service_account"`
	ArtifactsRootDir  string            `json:"artifacts_root_dir"`
	DefaultConfigPath string            `json:"default_config_path"`
}

type ObjectMeta struct {
	Name string `yaml:"name"`
}

type ClientConfigSpec struct {
	ContextName      string                      `yaml:"contextName"`
	KubeConfigSecret corev1.LocalObjectReference `yaml:"kubeConfigSecret"`
}

type ClientConfig struct {
	ApiVersion string           `yaml:"apiVersion"`
	Kind       string           `yaml:"kind"`
	Metadata   ObjectMeta       `yaml:"metadata"`
	Spec       ClientConfigSpec `yaml:"spec"`
}
