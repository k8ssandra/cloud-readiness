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
	_ "context"
	"fmt"
	"github.com/gruntwork-io/terratest/modules/files"
	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/gruntwork-io/terratest/modules/terraform"
	ts "github.com/gruntwork-io/terratest/modules/test-structure"
	"github.com/k8ssandra/cloud-readiness/k8ssandra/test/framework"
	"github.com/k8ssandra/cloud-readiness/k8ssandra/test/model"
	"github.com/stretchr/testify/require"
	"os"
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
	defaultCertManagerFile              = "https://github.com/jetstack/cert-manager/releases/download/v1.5.3/cert-manager.yaml"
	defaultCertManagerRepositoryName    = "jetstack"
	defaultCertManagerRepositoryURL     = "https://charts.jetstack.io"

	defaultRelativeRootFolder = "../.."
	prefixFolderName          = "cloud-k8c-"
)

func ProvisionMultiCluster(t *testing.T, readinessConfig model.ReadinessConfig) model.ProvisionMeta {

	uniqueProvisionId := random.UniqueId()
	provisionMeta := model.ProvisionMeta{
		KubeConfigs:      map[string]string{},
		ProvisionId:      uniqueProvisionId,
		ArtifactsRootDir: path.Join(os.TempDir(), prefixFolderName+uniqueProvisionId),
	}

	provConfig := readinessConfig.ProvisionConfig
	tfConfig := provConfig.TFConfig

	initTempArtifacts(t, provisionMeta)

	for name, ctx := range readinessConfig.Contexts {
		modulesFolder := ts.CopyTerraformFolderToTemp(t, defaultRelativeRootFolder, tfConfig.ModuleFolder)
		options := CreateOptions(readinessConfig, path.Join(modulesFolder, defaultTestSubFolder), provisionMeta.DefaultConfigPath)
		provisionCluster(t, name, ctx, readinessConfig,  options, provisionMeta, modulesFolder)
	}
	return provisionMeta
}

func initTempArtifacts(t *testing.T, meta model.ProvisionMeta) {

	var rootTempDir = meta.ArtifactsRootDir
	if files.IsExistingDir(rootTempDir) {
		err := os.Remove(rootTempDir)
		require.NoError(t, err, fmt.Sprintf("failed to remove a tmp root dir: %s", rootTempDir))
	}

	mkdirErr := os.MkdirAll(rootTempDir, defaultTempFilePerm)
	require.NoError(t, mkdirErr, fmt.Sprintf("failed to init folder: %s", rootTempDir))
}

func provisionCluster(t *testing.T, name string, ctx model.ContextConfig, config model.ReadinessConfig,
	tfOptions map[string]*terraform.Options, provisionMeta model.ProvisionMeta, tfModulesFolder string) {

	provConfig := config.ProvisionConfig
	if files.FileExists(provisionMeta.DefaultConfigPath) {
		logger.Log(t, fmt.Sprintf("backing up existing kube config file: %s", provisionMeta.DefaultConfigPath))
		cpErr:=files.CopyFile(provisionMeta.DefaultConfigPath, provisionMeta.DefaultConfigPath+"-backup" )
		require.NoError(t, cpErr, "expecting backup of default config file")
	}
	kubeOptions := k8s.NewKubectlOptions(name, 	provisionMeta.DefaultConfigPath, ctx.Namespace)
	logger.Log(t, fmt.Sprintf("kube config created: %s", 	provisionMeta.DefaultConfigPath))

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

func createHelmOptions(kubeConfig *k8s.KubectlOptions, values map[string]string, envs map[string]string) *helm.Options {

	helmOptions := &helm.Options{
		SetValues:      values,
		KubectlOptions: kubeConfig,
		EnvVars: envs,
	}
	return helmOptions
}

func deployK8ssandraCluster(t *testing.T, config model.ReadinessConfig, contextName string, options *k8s.KubectlOptions, namespace string) {
	logger.Log(t, fmt.Sprintf("deploying k8ssandra-cluster "+
		"for context: [%s] namespace: [%s]", contextName, namespace))

	k8cConfig := config.ProvisionConfig.K8cConfig
	k8cClusterFile := path.Join("../config/", k8cConfig.ValuesFilePath)
	out, err := k8s.RunKubectlAndGetOutputE(t, options,
		"apply", "-f", k8cClusterFile, "-n", namespace, "--validate=true")

	require.NoError(t, err)
	require.NotNil(t, out)
	logger.Log(t, out)
}

func Cleanup(t *testing.T, options *terraform.Options, testModulesPath string, kubectlConfigPath string) {
	logger.Log(t, "cleanup started")
	out := terraform.Destroy(t, options)
	logger.Log(t, fmt.Sprintf("destroy output: %s", out))
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
