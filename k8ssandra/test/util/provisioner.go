package util

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
	"encoding/base64"
	"fmt"
	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/gruntwork-io/terratest/modules/terraform"
	ts "github.com/gruntwork-io/terratest/modules/test-structure"
	"github.com/k8ssandra/cloud-readiness/k8ssandra/test/framework"
	"github.com/k8ssandra/cloud-readiness/k8ssandra/test/model"
	"github.com/k8ssandra/cloud-readiness/k8ssandra/test/util/cloud/gcp"
	"github.com/stretchr/testify/require"
	"log"
	"path"
	"strconv"
	"testing"
)

const (
	defaultTestSubFolder                = "env"
	defaultK8ssandraRepositoryName      = "k8ssandra"
	defaultK8ssandraOperatorReleaseName = "k8ssandra-operator"
	defaultK8ssandraOperatorChart       = "k8ssandra/k8ssandra-operator"
	defaultK8ssandraRepositoryURL       = "https://helm.k8ssandra.io/stable"
	defaultK8ssandraControlPlane        = "github.com/k8ssandra/config/deployments/control-plane"
	defaultK8ssandraDataPlane           = "github.com/k8ssandra/config/deployments/data-plane"
	defaultCertManagerFile              = "https://github.com/jetstack/cert-manager/releases/download/v1.5.3/cert-manager.yaml"
	defaultCertManagerRepositoryName    = "jetstack"
	defaultCertManagerRepositoryURL     = "https://charts.jetstack.io"
)

func ProvisionMultiCluster(t *testing.T, config model.ReadinessConfig) {

	provConfig := config.ProvisionConfig
	tfConfig := provConfig.TFConfig

	for name, ctx := range config.Contexts {
		tfModulesFolder := ts.CopyTerraformFolderToTemp(t, "../..", tfConfig.ModuleFolder)
		kubeConfigPath := k8s.CopyHomeKubeConfigToTemp(t)
		tfOptions := CreateOptions(t, config, path.Join(tfModulesFolder, defaultTestSubFolder), kubeConfigPath)
		provisionCluster(t, name, ctx, kubeConfigPath, tfOptions, config, tfModulesFolder)
	}
}

func provisionCluster(t *testing.T, name string, ctx model.ContextConfig, kubeConfigPath string,
	tfOptions map[string]*terraform.Options, config model.ReadinessConfig, tfModulesFolder string) {

	provConfig := config.ProvisionConfig
	kubeOptions := k8s.NewKubectlOptions(name, kubeConfigPath, ctx.Namespace)

	provisionSuccess := t.Run(fmt.Sprintf("provisioning: %s", name), func(t *testing.T) {
		t.Parallel()
		if provConfig.CleanOnly {
			clean(t, tfOptions[name], kubeOptions, tfModulesFolder, config)
		} else {
			apply(t, tfOptions[name], kubeOptions, tfModulesFolder, config)
		}
	})
	logger.Log(t, fmt.Sprintf("provision: %s result: %s", name, strconv.FormatBool(provisionSuccess)))
}

func nodeChecks(t *testing.T, name string, options *k8s.KubectlOptions, config model.ReadinessConfig) {

	provConfig := config.ProvisionConfig
	checkNodesSuccess := t.Run(fmt.Sprintf("provisioning: %s", name), func(t *testing.T) {
		t.Parallel()
		CheckNodesReady(t, options, config.ExpectedNodeCount,
			provConfig.DefaultRetries, provConfig.DefaultSleepSecs)
	})
	logger.Log(t, fmt.Sprintf("CheckNodesReady name:%s result: %s", name,
		strconv.FormatBool(checkNodesSuccess)))
}

func createHelmOptions(options *k8s.KubectlOptions, values map[string]string) *helm.Options {
	helmOptions := &helm.Options{
		SetValues:      values,
		KubectlOptions: options,
	}
	return helmOptions
}

func fetchServiceAccountConfig(t *testing.T, options map[string]*k8s.KubectlOptions, controlPlaneContext string,
	serviceAccount string) model.ServiceAccountConfig {

	serviceAccountSecret := fetchSecret(t, options[controlPlaneContext], serviceAccount)
	require.NotNil(t, serviceAccountSecret)
	logger.Log(t, fmt.Sprintf("service account [secret] obtained: %s", serviceAccountSecret))

	serviceAccountToken := fetchToken(t, options[controlPlaneContext], controlPlaneContext, serviceAccountSecret)
	require.NotNil(t, serviceAccountToken)
	logger.Log(t, fmt.Sprintf("service account [token] obtained: %s", serviceAccountToken))

	certificate := fetchCertificate(t, options[controlPlaneContext], controlPlaneContext, serviceAccountSecret)
	logger.Log(t, fmt.Sprintf("service account [cert] obtained: %s", certificate))

	return model.ServiceAccountConfig{}
}

func fetchCertificate(t *testing.T, options *k8s.KubectlOptions, contextName string, secret string) string {

	out, err := k8s.RunKubectlAndGetOutputE(t, options, "--context", contextName,
		"-n", options.Namespace, "get", "secret", secret, "-o", "jsonpath={.data['ca.crt']}")
	require.NoError(t, err)

	// TODO fix not getting ca.crt back from query
	// require.NotEmpty(t, out)
	return out
}

func fetchToken(t *testing.T, options *k8s.KubectlOptions, contextName string, secret string) string {

	out, err := k8s.RunKubectlAndGetOutputE(t, options, "--context", contextName,
		"-n", options.Namespace, "get", "secret", secret, "-o", "jsonpath={.data.token}")
	require.NoError(t, err)
	require.NotNil(t, out)

	decoded, err := base64.StdEncoding.DecodeString(out)
	if err != nil {
		log.Fatalf("Some error occured during base64 decode. Error %s", err.Error())
	}
	return string(decoded)
}

func fetchSecret(t *testing.T, options *k8s.KubectlOptions, serviceAccount string) string {
	out, err := k8s.RunKubectlAndGetOutputE(t, options, "--context", options.ContextName, "-n",
		options.Namespace, "get", "serviceaccount", serviceAccount, "-o", "jsonpath={.secrets[0].name}")
	require.NoError(t, err)
	require.NotNil(t, out)
	return out
}

func createKubeConfigs(t *testing.T, config model.ReadinessConfig) map[string]model.ContextOption {

	options := map[string]model.ContextOption{}

	for name, ctx := range config.Contexts {
		kubeConfigPath := k8s.CopyHomeKubeConfigToTemp(t)
		fn := gcp.ConstructFullContextName(name, config)
		kops := k8s.NewKubectlOptions(fn, kubeConfigPath, ctx.Namespace)
		options[name] = model.ContextOption{FullName: fn, KubeOptions: kops}
	}
	return options
}

func deployK8ssandraCluster(t *testing.T, config model.ReadinessConfig, contextName string, options *k8s.KubectlOptions) {
	logger.Log(t, fmt.Sprintf("deploying k8ssandra-cluster "+
		"for context: [%s] namespace: [%s]", contextName, options.Namespace))

	k8cConfig := config.ProvisionConfig.K8cConfig
	k8cClusterFile := path.Join("../config/", k8cConfig.ValuesFilePath)
	out, err := k8s.RunKubectlAndGetOutputE(t, options, "-n", options.Namespace,
		"apply", "-f", k8cClusterFile, "--validate=true")

	require.NoError(t, err)
	require.NotNil(t, out)
	logger.Log(t, out)
}

func Cleanup(t *testing.T, options *terraform.Options, testModulesPath string, kubectlConfigPath string) {
	logger.Log(t, "cleanup started")
	out := terraform.Destroy(t, options)
	logger.Log(t, fmt.Sprintf("destroy output: %s", out))

	removeTestKubeConfigFolder(t, kubectlConfigPath)
	removeTestDataFolder(t, testModulesPath)
}

func medusaSecretSetup(t *testing.T, kubectlOptions *k8s.KubectlOptions, k8cConfig model.K8cConfig, namespace string) {
	logger.Log(t, "creating medusa secret")

	// TODO - extract the raw json file using this ...
	// terraform output -raw service_account_key > medusa_gcp_key

	_, err := k8s.RunKubectlAndGetOutputE(t, kubectlOptions, "create secret generic ", k8cConfig.MedusaSecretName,
		" --from-file=", k8cConfig.MedusaSecretFromFile, "=", k8cConfig.MedusaSecretFromFileKey,
		" -n ", namespace)

	if err != nil {
		logger.Log(t, fmt.Sprintf("Failed to create Medusa generic secret: %s, referencing file: %s ",
			k8cConfig.MedusaSecretName, k8cConfig.MedusaSecretFromFile))
	}
	require.NoError(t, err)
}

func repoSetup(t *testing.T, helmOptions *helm.Options) bool {
	logger.Log(t, "setting up repository entries")
	helm.RemoveRepoE(t, helmOptions, defaultCertManagerRepositoryName)
	helm.AddRepo(t, helmOptions, defaultCertManagerRepositoryName, defaultCertManagerRepositoryURL)

	helm.RemoveRepoE(t, helmOptions, defaultK8ssandraRepositoryName)
	helm.AddRepo(t, helmOptions, defaultK8ssandraRepositoryName, defaultK8ssandraRepositoryURL)

	_, err := helm.RunHelmCommandAndGetStdOutE(t, helmOptions, "repo", "update")

	require.NoError(t, err)
	return true
}

func clean(t *testing.T, options *terraform.Options, kubectlOptions *k8s.KubectlOptions, testingModules string,
	config model.ReadinessConfig) {

	logger.Log(t, "clean requested")
	provConfig := config.ProvisionConfig

	terraform.Init(t, options)

	Cleanup(t, options, testingModules, kubectlOptions.ConfigPath)
	CheckNodesReady(t, kubectlOptions, 0,
		provConfig.DefaultRetries, provConfig.DefaultSleepSecs)
}

func apply(t *testing.T, options *terraform.Options, kubectlOptions *k8s.KubectlOptions, tfModulesFolder string,
	config model.ReadinessConfig) {

	provConfig := config.ProvisionConfig
	terraform.InitAndApply(t, options)

	if provConfig.PreTestCleanup {
		Cleanup(t, options, tfModulesFolder, kubectlOptions.ConfigPath)
	}
	if provConfig.PostTestCleanup {
		defer Cleanup(t, options, tfModulesFolder, kubectlOptions.ConfigPath)
	}
}

func verifyResourceDescriptors(t *testing.T, config model.ReadinessConfig) {
	require.NoError(t, framework.WaitForCondition(t, "established", "--timeout=240s", "--all", "crd"))
}

func createNamespace(t *testing.T, kubectlOptions *k8s.KubectlOptions, namespace string) {
	_, err := k8s.RunKubectlAndGetOutputE(t, kubectlOptions, "create ns ", namespace)
	if err != nil {
		logger.Log(t, fmt.Sprintf("failed create namespace: %s due to error: %s", namespace, err.Error()))
	}
}

func restartControlPlaneOperator(t *testing.T, options *k8s.KubectlOptions, config model.ReadinessConfig) {
	// TODO
	// kubectl -n k8ssandra-operator rollout restart deployment k8ssandra-operator-k8ssandra-operator
	//k8cConfig := config.ProvisionConfig.K8cConfig
	//k8cClusterFile := path.Join("../config/", k8cConfig.ValuesFilePath)
	//out, err := k8s.RunKubectlAndGetOutputE(t, options, "-n", options.Namespace, "rollout", "restart", "deployment")
	//
	//require.NoError(t, err)
	//require.NotNil(t, out)
	//logger.Log(t, out)
}

func verifyControlPlane(t *testing.T, context string) {
	// TODO
	// k8ssandra-operator-k8ssandra-operator -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="K8SSANDRA_CONTROL_PLANE")].value}'
}

func removeTestKubeConfigFolder(t *testing.T, kubectlConfigPath string) {
	// TODO
	//err := os.Remove(kubectlConfigPath)
	//require.NoError(t, err)
}

func removeTestDataFolder(t *testing.T, testModulesPath string) {
	// TODO
	// ts.CleanupTestDataFolder(t, testModulesPath)
}
