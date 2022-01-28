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

func TestK8cSmoke(t *testing.T) {

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
		ValuesFilePath:          "k8ssandra-clusters-v1.yaml",
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
		ServiceAccountNamePrefix: "sa",
		// Expected nodes per zone
		ExpectedNodeCount: 1,
		ProvisionConfig:   provisionConfig,
	}

	// util.ProvisionMultiCluster(t, k8cReadinessConfig)
	util.InstallK8ssandra(t, k8cReadinessConfig, "k8ssandra-operator")
}