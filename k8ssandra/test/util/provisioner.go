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
	"github.com/k8ssandra/cloud-readiness/k8ssandra/test/model"
	"github.com/k8ssandra/cloud-readiness/k8ssandra/test/util/cloud/gcp"
	"github.com/stretchr/testify/require"
	"os"
	"path"
	"strconv"
	"testing"
)

const (
	defaultTestSubFolder                = "env"
	defaultCassandraOperatorName        = "k8ssandra-operator-cass-operator"
	defaultK8ssandraRepositoryName      = "k8ssandra"
	defaultK8ssandraOperatorReleaseName = "k8ssandra-operator"
	defaultK8ssandraOperatorChart       = "k8ssandra/k8ssandra-operator"
	defaultK8ssandraRepositoryURL       = "https://helm.k8ssandra.io/stable"
	defaultCertManagerFile              = "https://github.com/jetstack/cert-manager/releases/download/v1.5.3/cert-manager.yaml"
	defaultCertManagerRepositoryName    = "jetstack"
	defaultCertManagerRepositoryURL     = "https://charts.jetstack.io"
	defaultTraefikRepositoryName        = "traefik"
	defaultTraefikRepositoryURL         = "https://helm.traefik.io/traefik"
	defaultTraefikChartName             = "traefik/traefik"
	defaultRelativeRootFolder           = "../.."
	prefixFolderName                    = "cloud-k8c-"

	DefaultAdminIdentifier = "K8C_ADMIN_ID"
	DefaultTraefikVersion  = "v10.3.2"
)

func ProvisionMultiCluster(t *testing.T, readinessConfig model.ReadinessConfig, provisionMeta model.ProvisionMeta) model.ProvisionMeta {

	uniqueProvisionId := random.UniqueId()
	testFolderName := path.Join(os.TempDir(), prefixFolderName+uniqueProvisionId)

	var meta = model.ProvisionMeta{
		KubeConfigs:       map[string]string{},
		Enable:            provisionMeta.Enable,
		ProvisionId:       uniqueProvisionId,
		ArtifactsRootDir:  testFolderName,
		ServiceAccount:    provisionMeta.ServiceAccount,
		DefaultConfigPath: provisionMeta.DefaultConfigPath,
		DefaultConfigDir:  provisionMeta.DefaultConfigDir,
		AdminIdentity:     DefaultAdminIdentifier,
	}

	provConfig := readinessConfig.ProvisionConfig
	tfConfig := provConfig.TFConfig

	initTempArtifacts(t, meta)

	for name, ctx := range readinessConfig.Contexts {

		testPath := ts.FormatTestDataPath(testFolderName, ctx.Name)
		logger.Log(t, fmt.Sprintf("test path formatted as: %s", testPath))

		modulesFolder := ts.CopyTerraformFolderToTemp(t, defaultRelativeRootFolder, tfConfig.ModuleFolder)

		options := CreateOptions(meta, readinessConfig, path.Join(modulesFolder, defaultTestSubFolder),
			meta.DefaultConfigPath)

		testData := model.ContextTestManifest{
			Name:          ctx.Name,
			ModulesFolder: modulesFolder,
		}

		identity := FetchEnv(t, meta.AdminIdentity)
		env := CreateIdentityEnv(meta.DefaultConfigPath, identity, ctx.CloudConfig.CredPath)

		gcp.Switch(t, identity, env)
		ts.SaveTestData(t, testPath, testData)

		provisionCluster(t, name, options, meta)
	}
	return meta
}

func Cleanup(t *testing.T, meta model.ProvisionMeta, name string, options map[string]*terraform.Options) bool {

	logger.Log(t, fmt.Sprintf("cleanup started for resources in: %s", name))

	if meta.Enable.Simulate {
		logger.Log(t, fmt.Sprintf("SIMULATE cleanup of cloud resources returning success."))
		return true
	}

	initPlanOut, initPlanErr := terraform.InitAndPlanE(t, options[name])
	if initPlanErr != nil {
		logger.Log(t, fmt.Sprintf("failed cleanup on init-plan, error: %s", initPlanErr.Error()))
		return false
	}
	logger.Log(t, fmt.Sprintf("successful cleanup init-plan, output: %s", initPlanOut))

	destroyOut, destroyErr := terraform.DestroyE(t, options[name])
	if destroyErr != nil {
		logger.Log(t, fmt.Sprintf("failed cleanup destroy, error: %s", destroyErr.Error()))
	} else {
		logger.Log(t, fmt.Sprintf("successful cleanup destroy, output: %s", destroyOut))
	}
	return destroyErr == nil
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

func provisionCluster(t *testing.T, name string, tfOptions map[string]*terraform.Options,
	meta model.ProvisionMeta) {

	if files.FileExists(meta.DefaultConfigPath) {
		logger.Log(t, fmt.Sprintf("backing up existing kube config file: %s", meta.DefaultConfigPath))
		cpErr := files.CopyFile(meta.DefaultConfigPath, meta.DefaultConfigPath+"-backup")
		require.NoError(t, cpErr, "expecting backup of default config file")
	}

	provisionSuccess := t.Run(name, func(t *testing.T) {
		t.Parallel()
		if meta.Enable.Simulate {
			logger.Log(t, fmt.Sprintf("SIMULATION, the provisioning init & apply is not "+
				"being invoked for %s", t.Name()))
		} else {
			logger.Log(t, fmt.Sprintf("apply being invoked for name: %s", name))
			apply(t, tfOptions[name])
		}
	})
	logger.Log(t, fmt.Sprintf("provision: %s success: %s", name, strconv.FormatBool(provisionSuccess)))
}

func createHelmOptions(kubeConfig *k8s.KubectlOptions, values map[string]string, envs map[string]string,
	isSimulate bool) *helm.Options {

	var extraArgs = map[string][]string{}
	if isSimulate {
		extraArgs["install"] = []string{"--debug", "--dry-run"}
	}

	helmOptions := &helm.Options{
		SetValues:      values,
		KubectlOptions: kubeConfig,
		EnvVars:        envs,
		ExtraArgs:      extraArgs,
	}

	return helmOptions
}

func apply(t *testing.T, options *terraform.Options) {

	terraform.InitAndPlan(t, options)
	logger.Log(t, fmt.Sprintf("initialized and planned: %s", t.Name()))

	terraform.Apply(t, options)
	logger.Log(t, fmt.Sprintf("applied: %s", t.Name()))

}
