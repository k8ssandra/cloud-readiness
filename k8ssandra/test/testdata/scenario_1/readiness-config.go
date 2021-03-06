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

package scenario_1

import (
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/k8ssandra/cloud-readiness/k8ssandra/test/model"
	"github.com/k8ssandra/cloud-readiness/k8ssandra/test/util"
	"strings"
	"testing"
)

func ReadinessConfig(t *testing.T, contexts map[string]model.ContextConfig) (model.ProvisionMeta, model.ReadinessConfig) {

	configRootDir, configPath := util.FetchKubeConfigPath(t)

	var enablement = model.EnableConfig{
		Simulate:        false,
		RemoveAll:       false,
		Install:         false,
		ProvisionInfra:  false,
		PreInstallSetup: true,
	}

	var provisionMeta = model.ProvisionMeta{
		Enable:            enablement,
		ProvisionId:       "k8c-whwimk",
		ArtifactsRootDir:  "/tmp/cloud-k8c-whwimk",
		KubeConfigs:       nil,
		DefaultConfigPath: configPath,
		DefaultConfigDir:  configRootDir,
		AdminIdentity:     util.DefaultAdminIdentifier,
	}

	k8cConfig := model.K8cConfig{
		ClusterName:             "bootz-k8c-cluster",
		ValuesFilePath:          "k8c-multi-dc.yaml",
		MedusaSecretName:        "dev-k8ssandra-medusa-key",
		MedusaSecretFromFileKey: "medusa_gcp_key",
		MedusaSecretFromFile:    "medusa_gcp_key.json",
		ClusterScoped:           false,
	}

	tfConfig := model.TFConfig{
		ModuleFolder: "./provision/gcp",
	}

	helmConfig := model.HelmConfig{
		ChartPath: "k8ssandra/k8ssandra",
	}

	provisionConfig := model.ProvisionConfig{
		TFConfig:           tfConfig,
		HelmConfig:         helmConfig,
		K8cConfig:          k8cConfig,
		DefaultSleepSecs:   20,
		DefaultRetries:     30,
		DefaultTimeoutSecs: 240,
	}

	readinessConfig := model.ReadinessConfig{
		UniqueId:                 strings.ToLower(random.UniqueId()),
		Contexts:                 contexts,
		ServiceAccountNameSuffix: "sa",

		// Expected nodes per zone
		ExpectedNodeCount: 2,
		ProvisionConfig:   provisionConfig,
	}

	return provisionMeta, readinessConfig
}
