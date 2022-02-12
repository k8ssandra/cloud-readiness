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

func TestK8cSmokeTemplate(t *testing.T) {

	configRootDir, configPath := util.FetchKubeConfigPath(t)

	// When enabled, utilizes an existing set provisioned clusters
	var meta = ProvisionMeta{
		Enabled:           false,
		ProvisionId:       "FpWdV6",
		KubeConfigs:       nil,
		ServiceAccount:    "",
		ArtifactsRootDir:  "<reference-dir example: /tmp/cloud-k8c-FpWdV6>",
		DefaultConfigPath: configPath,
		DefaultConfigDir:  configRootDir,
		AdminIdentity:     "K8C_ADMIN_ID",
	}

	cloudConfig := CloudConfig{
		Project:     "community-ecosystem",
		Region:      "us-central1",
		Location:    "us-central1-a",
		Environment: "dev",
		MachineType: "e2-highcpu-8",
		CredPath:    "<your-path-to-creds>",
		CredKey:     "<example: GOOGLE_APPLICATION_CREDENTIALS>",
		Bucket:      "google_storage_bucket",
	}

	k8cConfig := K8cConfig{
		MedusaSecretName:        "dev-k8ssandra-medusa-key",
		MedusaSecretFromFileKey: "medusa_gcp_key",
		MedusaSecretFromFile:    "medusa_gcp_key.json",
		ValuesFilePath:          "k8ssandra-clusters-v2.yaml",
		ClusterScoped:           false,
	}

	tfConfig := TFConfig{
		ModuleFolder: "./provision/gcp",
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
		Name:          "<ctx-name-1>",
		Namespace:     "<your-ns>",
		ClusterLabels: []string{"control-plane"},
	}

	ctxConfig2 := ContextConfig{
		Name:          "<ctx-name-2>",
		Namespace:     "<your-ns>",
		ClusterLabels: []string{"data-plane"},
	}

	ctxConfig3 := ContextConfig{
		Name:          "<ctx-name-3>",
		Namespace:     "<your-ns>",
		ClusterLabels: []string{"data-plane"},
	}

	contexts := map[string]ContextConfig{ctxConfig1.Name: ctxConfig1,
		ctxConfig2.Name: ctxConfig2, ctxConfig3.Name: ctxConfig3}

	k8cReadinessConfig := ReadinessConfig{
		UniqueId:                 strings.ToLower(random.UniqueId()),
		Contexts:                 contexts,
		ServiceAccountNameSuffix: "sa",
		// Expected nodes per zone
		ExpectedNodeCount: 1,
		ProvisionConfig:   provisionConfig,
	}

	util.Apply(t, meta, k8cReadinessConfig)
}
