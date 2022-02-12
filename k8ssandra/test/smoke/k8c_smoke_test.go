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
	"fmt"
	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/gruntwork-io/terratest/modules/random"
	. "github.com/k8ssandra/cloud-readiness/k8ssandra/test/model"
	"github.com/k8ssandra/cloud-readiness/k8ssandra/test/util"
	_ "github.com/k8ssandra/cloud-readiness/k8ssandra/test/util"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestK8cSmoke(t *testing.T) {

	configRootDir, configPath := util.FetchKubeConfigPath(t)
	// when enabled, utilize an existing set of kubeconfigs related to context short-names
	var provisionMeta = ProvisionMeta{
		Enabled:           true,
		ProvisionId:       "4LNs0p",
		KubeConfigs:       nil,
		ServiceAccount:    "",
		ArtifactsRootDir:  "/tmp/cloud-k8c-4LNs0p",
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
		CredPath:    "/home/jbanks/.config/gcloud/application_default_credentials.json",
		CredKey:     "GOOGLE_APPLICATION_CREDENTIALS",
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
		Name:          "bootz100",
		Namespace:     "bootz",
		ClusterLabels: []string{"control-plane"},
	}

	ctxConfig2 := ContextConfig{
		Name:          "bootz101",
		Namespace:     "bootz",
		ClusterLabels: []string{"data-plane"},
	}

	ctxConfig3 := ContextConfig{
		Name:          "bootz102",
		Namespace:     "bootz",
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

	if !provisionMeta.Enabled {
		logger.Log(t, "an infrastructure provisioning is not being referenced, infrastructure provision started ...")
		provisionMeta = util.ProvisionMultiCluster(t, k8cReadinessConfig)
		require.NotEmpty(t, provisionMeta.ProvisionId, "expected provision step to occur.")
		logger.Log(t, fmt.Sprintf("provision submitted for identifier: %s", provisionMeta.ProvisionId))
	} else {
		logger.Log(t, fmt.Sprintf("found an existing infrastructure to reference, identifier: %s", provisionMeta.ProvisionId))
	}

	logger.Log(t, fmt.Sprintf("installation starting for provision identifier: %s", provisionMeta.ProvisionId))
	util.InstallK8ssandra(t, k8cReadinessConfig, provisionMeta)
}
