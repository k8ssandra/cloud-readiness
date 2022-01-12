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

package model

type CloudConfig struct {
	Type        string
	Location    string
	Region      string
	Project     string
	Name        string
	CredPath    string
	CredKey     string
	Environment string
	MachineType string
	Bucket      string
}
type TFConfig struct {
	ModuleFolder string
}
type HelmConfig struct {
	ChartPath string
}

type K8cConfig struct {
	Version                 string
	MedusaSecretName        string
	MedusaSecretFromFileKey string
	MedusaSecretFromFile    string
	ValuesFilePath          string
	ClusterScoped			bool
}

type ProvisionConfig struct {
	PreTestCleanup     bool
	PostTestCleanup    bool
	CleanOnly          bool
	CleanDir           string
	DefaultRetries     int
	DefaultSleepSecs   int
	DefaultTimeoutSecs int
	HelmConfig         HelmConfig
	TFConfig           TFConfig
	CloudConfig        CloudConfig
	K8cConfig          K8cConfig
}

type ProvisionResult struct {
	Success bool
}

type ContextConfig struct {
	Name          string
	Namespace     string
	ClusterLabels []string
}

type ReadinessConfig struct {
	ProvisionConfig          ProvisionConfig
	KubectlConfigPath        string
	UniqueId                 string
	RootFolder               string
	Contexts                 map[string]ContextConfig
	ServiceAccountNamePrefix string
	ExpectedNodeCount        int
}

type ServiceAccountConfig struct {

}
