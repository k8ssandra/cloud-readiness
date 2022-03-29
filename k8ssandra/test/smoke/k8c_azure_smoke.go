package smoke

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
	"github.com/gruntwork-io/terratest/modules/random"
	. "github.com/k8ssandra/cloud-readiness/k8ssandra/test/model"
	"github.com/k8ssandra/cloud-readiness/k8ssandra/test/util"
	_ "github.com/k8ssandra/cloud-readiness/k8ssandra/test/util"
	"strings"
	"testing"
)

func TestK8cAzureSmoke(t *testing.T) {

	configRootDir, configPath := util.FetchKubeConfigPath(t)

	// Enable to utilize an existing set of cloud infrastructure artifacts already existing.
	// The ProvisionId and ArtifactsRootDir must be supplied with accurate information.
	// When not-enabled, will provision fresh cloud infrastructure based on model values.
	var provisionMeta = ProvisionMeta{
		Enabled:           false,
		RemoveAll:         false,
		ProvisionId:       "k8c-fTNba1",
		ArtifactsRootDir:  "/tmp/cloud-k8c-fTNba1",
		KubeConfigs:       nil,
		ServiceAccount:    "",
		DefaultConfigPath: configPath,
		DefaultConfigDir:  configRootDir,
		AdminIdentity:     util.DefaultAdminIdentifier,
	}

	cloudConfig := CloudConfig{
		Project:     "community-ecosystem",
		Region:      "us-central1",
		Location:    "us-central1-a",
		Environment: "dev",
		MachineType: "e2-highcpu-8",
		CredPath:    "", // "/home/jbanks/.config/gcloud/application_default_credentials.json",
		CredKey:     "", // "GOOGLE_APPLICATION_CREDENTIALS",
		Bucket:      "azure_storage_bucket",
	}

	k8cConfig := K8cConfig{
		MedusaSecretName:        "dev-k8ssandra-medusa-key",
		MedusaSecretFromFileKey: "medusa_azure_key",
		MedusaSecretFromFile:    "medusa_azure_key.json",
		ClusterName:             "bootz-k8c-cluster",
		ValuesFilePath:          "k8ssandra-clusters-v2.yaml",
		ClusterScoped:           false,
	}

	tfConfig := TFConfig{
		ModuleFolder: "./provision/azure",
	}

	helmConfig := HelmConfig{
		ChartPath: "k8ssandra/k8ssandra",
	}

	provisionConfig := ProvisionConfig{
		CleanOnly:          false,
		CleanDir:           "<as-needed>",
		PreTestCleanup:     false,
		PostTestCleanup:    false,
		TFConfig:           tfConfig,
		HelmConfig:         helmConfig,
		CloudConfig:        cloudConfig,
		K8cConfig:          k8cConfig,
		DefaultSleepSecs:   20,
		DefaultRetries:     30,
		DefaultTimeoutSecs: 240,
	}

	ctxConfig1 := ContextConfig{
		Name:          "bootz900",
		Namespace:     "bootz",
		ClusterLabels: []string{"control-plane"},
	}

	ctxConfig2 := ContextConfig{
		Name:          "bootz901",
		Namespace:     "bootz",
		ClusterLabels: []string{"data-plane"},
	}

	ctxConfig3 := ContextConfig{
		Name:          "bootz902",
		Namespace:     "bootz",
		ClusterLabels: []string{"data-plane"},
	}

	contexts := map[string]ContextConfig{
		ctxConfig1.Name: ctxConfig1,
		ctxConfig2.Name: ctxConfig2,
		ctxConfig3.Name: ctxConfig3,
	}

	k8cReadinessConfig := ReadinessConfig{
		UniqueId:                 strings.ToLower(random.UniqueId()),
		Contexts:                 contexts,
		ServiceAccountNameSuffix: "sa",
		// Expected nodes per zone
		ExpectedNodeCount: 2,
		ProvisionConfig:   provisionConfig,
	}

	util.Apply(t, provisionMeta, k8cReadinessConfig)
}
