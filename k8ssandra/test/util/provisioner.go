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
	"strings"
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

func ProvisionMultiCluster(t *testing.T, readinessConfig model.ReadinessConfig,
	provisionMeta model.ProvisionMeta) model.ProvisionMeta {

	uniqueProvisionId := strings.ToLower(random.UniqueId())
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

		provisionCluster(t, name, options, meta, readinessConfig)
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
	meta model.ProvisionMeta, readinessConfig model.ReadinessConfig) {

	if files.FileExists(meta.DefaultConfigPath) {
		logger.Log(t, fmt.Sprintf("backing up existing kube config file: %s", meta.DefaultConfigPath))
		cpErr := files.CopyFile(meta.DefaultConfigPath, meta.DefaultConfigPath+"-backup")
		require.NoError(t, cpErr, "expecting backup of default config file")
	}

	// t.Setenv("test.timeout", 0)
	t.Setenv("test.v", "true")
	t.Setenv("test.trace", path.Join(meta.ArtifactsRootDir, "trace-x"))

	timeout, _ := t.Deadline()
	logger.Log(t, fmt.Sprintf("executing test:"+
		"%s with timeout: %d(m)",
		t.Name(), timeout.UnixMilli()))

	testRun := t.Run(name, func(t *testing.T) {
		t.Parallel()
		if meta.Enable.Simulate {
			timeout, _ := t.Deadline()
			logger.Log(t, fmt.Sprintf("SIMULATION, init, plan, and apply being invoked for:"+
				"%s with timeout: %d(m)", t.Name(), timeout.UnixMilli()))

		} else {
			logger.Log(t, fmt.Sprintf("init, plan and apply being invoked for: %s", name))
			planErr, applyErr := apply(t, tfOptions[name])

			if planErr != nil || applyErr != nil {
				logger.Log(t, fmt.Sprintf("provision: %s, failure discovered. plan err reported: %s apply "+
					"err reported: %s", name, planErr, applyErr))
				// TODO indicate to the test client a failure overall, IF we can determine that there is an actual
				// issue with the TF activities or it was simply a timeout on that side.
			}
		}

		if meta.Enable.PreInstallSetup {
			if meta.Enable.Simulate {
				logger.Log(t, fmt.Sprintf("SIMULATION, post install resources requested for: %s", name))
			} else {
				logger.Log(t, fmt.Sprintf("pre-install setup requested for: %s", name))
				InstallSetup(t, meta, readinessConfig)
			}
		}
	})
	logger.Log(t, fmt.Sprintf("test run: %s reported success as: %s", name, strconv.FormatBool(testRun)))

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

func apply(t *testing.T, options *terraform.Options) (error, error) {

	_, initPlanErr := terraform.InitAndPlanE(t, options)
	logger.Log(t, fmt.Sprintf("initialized and planned: %s", t.Name()))

	_, applyErr := terraform.ApplyE(t, options)
	logger.Log(t, fmt.Sprintf("applied: %s", t.Name()))

	return initPlanErr, applyErr
}
