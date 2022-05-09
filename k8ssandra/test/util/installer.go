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
	"fmt"
	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/k8ssandra/cloud-readiness/k8ssandra/test/model"
	"github.com/k8ssandra/cloud-readiness/k8ssandra/test/util/cloud/gcp"
	"github.com/stretchr/testify/require"
	_ "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/utils/strings/slices"
	"os"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	defaultTempFilePerm = os.FileMode(0700)
	defaultTimeout      = time.Second * 5
	defaultInterval     = time.Millisecond * 250

	defaultK8ssandraSecret     = "k8s-contexts"
	defaultIdentityDomain      = "@community-ecosystem.iam.gserviceaccount.com"
	defaultControlPlaneKey     = "K8SSANDRA_CONTROL_PLANE"
	defaultWebhookServiceName  = "webhook-service"
	defaultControlPlaneLabel   = "control-plane"
	defaultKubeConfigFileName  = "kubeconfig"
	defaultTraefikResourceName = "traefik"
	helmInstallDryRun          = "--dry-run"
)

func InstallK8ssandra(t *testing.T, readinessConfig model.ReadinessConfig, meta model.ProvisionMeta) {

	logger.Log(t, "\n\ninstallation started")
	options := installSetup(t, meta, readinessConfig)

	installControlPlaneOperator(t, meta, readinessConfig, options)
	installDataPlaneOperators(t, meta, readinessConfig, options)

	CreateClientConfigurations(t, meta, readinessConfig, options)
	installK8ssandraCluster(t, meta, readinessConfig, options)
}

func installDataPlaneOperators(t *testing.T, meta model.ProvisionMeta, readinessConfig model.ReadinessConfig, ctxOptions map[string]model.ContextOption) {

	logger.Log(t, "\n\ninstallation of data-plane")

	var isClusterScoped = readinessConfig.ProvisionConfig.K8cConfig.ClusterScoped

	for name, ctxConfig := range readinessConfig.Contexts {

		kubeConfig := ctxOptions[name].KubectlOptions
		SetCurrentContext(t, ctxOptions[name].FullName, kubeConfig)
		isControlPlane := IsControlPlane(ctxConfig)

		if !isControlPlane {
			helmOptions := createHelmOptions(kubeConfig, map[string]string{
				defaultControlPlaneKey: strconv.FormatBool(isControlPlane)}, kubeConfig.Env, meta.Enable.Simulate)

			logger.Log(t, fmt.Sprintf("installing k8ssandra-operator on data-plane: %s", name))
			installK8ssandraOperator(t, helmOptions, ctxConfig.Name, ctxConfig.Namespace, isClusterScoped, isControlPlane)
		}
	}
}

func installK8ssandraCluster(t *testing.T, meta model.ProvisionMeta, readinessConfig model.ReadinessConfig,
	ctxOptions map[string]model.ContextOption) {

	logger.Log(t, "\n\ninstallation of cluster")
	for name, ctxConfig := range readinessConfig.Contexts {
		kubeConfig := ctxOptions[name].KubectlOptions
		SetCurrentContext(t, ctxOptions[name].FullName, kubeConfig)

		if IsControlPlane(ctxConfig) {

			kubeConfig.Env[defaultControlPlaneKey] = "true"

			if meta.Enable.Simulate {
				logger.Log(t, fmt.Sprintf("=== SIMULATE deploying k8ssandra-cluster on control plane: %s", name))
				continue
			}

			logger.Log(t, fmt.Sprintf("=== deploying k8ssandra-cluster on control plane: %s", name))
			require.Eventually(t, func() bool {
				endpointIP := WaitForEndpoint(t, kubeConfig, defaultK8ssandraOperatorReleaseName+"-"+defaultWebhookServiceName)
				logger.Log(t, fmt.Sprintf("endpoint discovery on control-plane: %s", endpointIP))
				return strings.TrimSpace(endpointIP) != "''"
			}, time.Second*30, defaultInterval, "timeout waiting for endpoint ip to exist")

			logger.Log(t, "\n\nK8ssandra: control-plane k8c cluster deployment underway ...")
			deployK8ssandraCluster(t, readinessConfig, ctxConfig.Name, kubeConfig, ctxConfig.Namespace, meta.Enable.Simulate)

			time.Sleep(defaultTimeout * 6)
		}
	}
}

func deployK8ssandraCluster(t *testing.T, config model.ReadinessConfig, contextName string,
	options *k8s.KubectlOptions, namespace string, isSimulate bool) bool {
	logger.Log(t, fmt.Sprintf("deploying k8ssandra-cluster for context: [%s] namespace: [%s]",
		contextName, namespace))

	if isSimulate {
		logger.Log(t, "\n\nSIMULATE deploy of k8ssandra-cluster")
		return true
	}

	k8cConfig := config.ProvisionConfig.K8cConfig
	_, err := k8s.RunKubectlAndGetOutputE(t, options, "apply", "-f",
		path.Join("../config/", k8cConfig.ValuesFilePath), "-n", namespace)

	return err == nil
}

func installControlPlaneOperator(t *testing.T, meta model.ProvisionMeta, readinessConfig model.ReadinessConfig,
	ctxOptions map[string]model.ContextOption) string {

	logger.Log(t, "\n\ninstalling control-plane")
	var isClusterScoped = readinessConfig.ProvisionConfig.K8cConfig.ClusterScoped
	var controlPlaneContextName = ""

	for name, ctxConfig := range readinessConfig.Contexts {
		kubeConfig := ctxOptions[name].KubectlOptions
		SetCurrentContext(t, ctxOptions[name].FullName, kubeConfig)

		isControlPlane := IsControlPlane(ctxConfig)
		if isControlPlane {
			setEnvErr := os.Setenv(defaultControlPlaneKey, strconv.FormatBool(isControlPlane))
			require.NoError(t, setEnvErr)

			kubeConfig.Env[defaultControlPlaneKey] = strconv.FormatBool(isControlPlane)
			helmOptions := createHelmOptions(kubeConfig, map[string]string{
				defaultControlPlaneKey: strconv.FormatBool(isControlPlane)}, kubeConfig.Env, meta.Enable.Simulate)

			installK8ssandraOperator(t, helmOptions, ctxConfig.Name, ctxConfig.Namespace, isClusterScoped, isControlPlane)
			controlPlaneContextName = ctxOptions[name].FullName
		}
	}
	return controlPlaneContextName
}

func installCertManager(t *testing.T, options *k8s.KubectlOptions, isSimulate bool) {

	if isSimulate {
		logger.Log(t, "SIMULATE install cert manager ...")
		return
	}

	// Necessary as the cert manager configuration currently used, specifies its own namespaces
	withoutNamespace := &options
	(*withoutNamespace).Namespace = ""
	(*withoutNamespace).Env = map[string]string{"installCRDs": "true"}

	_, err := k8s.RunKubectlAndGetOutputE(t, *withoutNamespace,
		"apply", "-f", defaultCertManagerFile)

	if err != nil {
		logger.Log(t, "retrying install cert manager ...")
		_, err2 := k8s.RunKubectlAndGetOutputE(t, *withoutNamespace,
			"apply", "-f", defaultCertManagerFile)
		require.NoError(t, err2)
	}
}

func installK8ssandraOperator(t *testing.T, options *helm.Options, contextName string, namespace string,
	isClusterScoped bool, isControlPlane bool) {

	options.KubectlOptions.Namespace = namespace

	logger.Log(t, fmt.Sprintf("installing k8ssandra-operator "+
		"for context: %s and namespace: %s", contextName, namespace))
	logger.Log(t, fmt.Sprintf("cluster scoped for k8ssandra-operator is set as: %s",
		strconv.FormatBool(isClusterScoped)))

	var result, err = helmInstall(t, options, defaultK8ssandraOperatorReleaseName, defaultK8ssandraOperatorChart, namespace)
	if err != nil {
		logger.Log(t, fmt.Sprintf("failed k8ssandra-operator install due to error: %s", err.Error()))
		if strings.Contains(err.Error(), "cannot re-use a name") {
			uninstallK8ssandraOperator(t, options)
			result, err = helmInstall(t, options, defaultK8ssandraOperatorReleaseName, defaultK8ssandraOperatorChart, namespace)
		}
	}

	if !isControlPlane {
		if slices.Contains(options.ExtraArgs["install"], helmInstallDryRun) {
			logger.Log(t, "SIMULATE checking pod availability for k8ssandra-operator along with patching K8SSANDRA_CONTROL_PLANE=false")
		} else {
			patchContent := "{\"spec\": {\"containers\": [{\"env\": [{\"name\":\"K8SSANDRA_CONTROL_PLANE\",\"value\":\"false\"}]}]}}"
			/*
				 CONTROL_PLANE: kind-k8ssandra-0
				2022-04-21T21:43:24.2506565Z   DATA_PLANES: kind-k8ssandra-1,kind-k8ssandra-2
			*/
			isRunning, podName := IsPodRunning(t, options.KubectlOptions, "k8ssandra-operator")
			if isRunning {
				out, err := k8s.RunKubectlAndGetOutputE(t, options.KubectlOptions, "patch", podName, "-p", patchContent)
				require.NoError(t, err, "failed to apply patch content on data-plane")

				logger.Log(t, fmt.Sprintf("restarting operator, patch complete: %s", out))
				RestartOperator(t, options.KubectlOptions.Namespace, options.KubectlOptions)
			} else {
				logger.Log(t, fmt.Sprintf("k8ssandra-operator pod: %s is NOT available", podName))
			}
		}
	}

	require.NoError(t, err, "unexpected error during k8ssandra-operator installation")
	logger.Log(t, fmt.Sprintf("installation result: %s", result))

}

func installSetup(t *testing.T, meta model.ProvisionMeta, readinessConfig model.ReadinessConfig) map[string]model.ContextOption {

	identity := FetchEnv(t, meta.AdminIdentity)
	require.NotEmpty(t, identity, "expecting identity to be provided to apply preconditions")

	var contextConfigs = map[string]*k8s.KubectlOptions{}
	var isRepoSetup = false

	for name, ctx := range readinessConfig.Contexts {

		logger.Log(t, fmt.Sprintf("installation setup for: %s", name))

		env := CreateIdentityEnv(meta.DefaultConfigPath, identity, ctx.CloudConfig.CredPath)
		gcp.Switch(t, identity, env)

		fullName := gcp.ConstructFullContextName(name, ctx.CloudConfig)
		gcp.FetchCreds(t, ctx.CloudConfig, env, gcp.ConstructCloudClusterName(name, ctx.CloudConfig))
		kubeConfig := k8s.NewKubectlOptions(fullName, meta.DefaultConfigPath,
			readinessConfig.Contexts[name].Namespace)
		SetCurrentContext(t, fullName, kubeConfig)

		helmOptions := createHelmOptions(kubeConfig, map[string]string{}, map[string]string{},
			meta.Enable.Simulate)

		if !isRepoSetup {
			isRepoSetup = repoSetup(t, helmOptions)
		}

		installCertManager(t, kubeConfig, meta.Enable.Simulate)
		contextConfigs[name] = kubeConfig

		installTraefik(t, helmOptions, readinessConfig.Contexts[name], meta.Enable.Simulate)
	}

	return CreateContextOptions(t, readinessConfig, meta, contextConfigs)
}

func repoSetup(t *testing.T, helmOptions *helm.Options) bool {
	logger.Log(t, "setting up repository entries")

	rse1 := helm.RemoveRepoE(t, helmOptions, defaultCertManagerRepositoryName)
	if rse1 != nil {
		logger.Log(t, fmt.Sprintf("WARNING: failure encountered during attempted repo removal. %s", rse1.Error()))
	}
	helm.AddRepo(t, helmOptions, defaultCertManagerRepositoryName, defaultCertManagerRepositoryURL)

	rse2 := helm.RemoveRepoE(t, helmOptions, defaultK8ssandraRepositoryName)
	if rse2 != nil {
		logger.Log(t, fmt.Sprintf("WARNING: failure encountered during attempted repo removal. %s", rse2.Error()))
	}
	helm.AddRepo(t, helmOptions, defaultK8ssandraRepositoryName, defaultK8ssandraRepositoryURL)

	rse3 := helm.RemoveRepoE(t, helmOptions, defaultTraefikRepositoryName)
	if rse3 != nil {
		logger.Log(t, fmt.Sprintf("WARNING: failure encountered during attempted repo removal. %s", rse3.Error()))
	}
	helm.AddRepo(t, helmOptions, defaultTraefikRepositoryName, defaultTraefikRepositoryURL)

	_, err := helm.RunHelmCommandAndGetStdOutE(t, helmOptions, "repo", "update")

	require.NoError(t, err)
	return true
}

func installTraefik(t *testing.T, helmOptions *helm.Options, config model.ContextConfig, isSimulate bool) {

	require.NotNil(t, helmOptions, "expecting helm options to install traefik")
	require.NotNil(t, config, "expecting readiness config to install traefik")

	if isSimulate {
		logger.Log(t, "SIMULATE install Traefik")
		return
	}

	withoutNamespace := &helmOptions.KubectlOptions
	(*withoutNamespace).Namespace = ""

	DeleteResource(t, *withoutNamespace, "ClusterRoleBinding", defaultTraefikResourceName)
	DeleteResource(t, *withoutNamespace, "ClusterRole", defaultTraefikResourceName)

	_, _ = uninstallTraefik(t, helmOptions)

	version := config.NetworkConfig.TraefikVersion
	filePath := path.Join("../config/", config.NetworkConfig.TraefikValuesFile)
	_, err := helmInstallFromFile(t, helmOptions, defaultTraefikRepositoryName, defaultTraefikChartName, version, filePath)

	require.NoError(t, err, "expecting that Traefik can be installed")
}

func helmInstall(t *testing.T, options *helm.Options, releaseName string, chart string, namespace string) (string, error) {

	if slices.Contains(options.ExtraArgs["install"], helmInstallDryRun) {
		return helm.RunHelmCommandAndGetOutputE(t, options, "install", releaseName, chart,
			"-n", namespace, "--create-namespace", options.ExtraArgs["install"][0], options.ExtraArgs["install"][1])
	}
	return helm.RunHelmCommandAndGetOutputE(t, options, "install", releaseName, chart,
		"-n", namespace, "--create-namespace")
}

func helmInstallFromFile(t *testing.T, options *helm.Options, name string, chart string, version string,
	filePath string) (string, error) {

	if slices.Contains(options.ExtraArgs["install"], helmInstallDryRun) {
		return helm.RunHelmCommandAndGetStdOutE(t, options, "install", name, chart,
			"--version", version, "-f", filePath, options.ExtraArgs["install"][0], options.ExtraArgs["install"][1])
	}
	return helm.RunHelmCommandAndGetStdOutE(t, options, "install", name, chart,
		"--version", version, "-f", filePath)
}

func uninstallTraefik(t *testing.T, helmOptions *helm.Options) (string, error) {

	if slices.Contains(helmOptions.ExtraArgs["install"], helmInstallDryRun) {
		return helm.RunHelmCommandAndGetOutputE(t, helmOptions, "uninstall", defaultTraefikRepositoryName,
			helmOptions.ExtraArgs["install"][0], helmOptions.ExtraArgs["install"][1])
	}
	return helm.RunHelmCommandAndGetOutputE(t, helmOptions, "uninstall", defaultTraefikRepositoryName)
}

func uninstallK8ssandraOperator(t *testing.T, helmOptions *helm.Options) {

	if slices.Contains(helmOptions.ExtraArgs["install"], helmInstallDryRun) {
		var _, err = helm.RunHelmCommandAndGetStdOutE(t, helmOptions, "uninstall", defaultK8ssandraOperatorReleaseName,
			"-n", helmOptions.KubectlOptions.Namespace, helmOptions.ExtraArgs["install"][0], helmOptions.ExtraArgs["install"][1])
		if err != nil {
			logger.Log(t, fmt.Sprintf("WARNING: failure encountered during attempted k8ssandra-operator uninstall. %s", err.Error()))
		}

	} else {
		var _, err = helm.RunHelmCommandAndGetStdOutE(t, helmOptions, "uninstall", defaultK8ssandraOperatorReleaseName,
			"-n", helmOptions.KubectlOptions.Namespace)
		if err != nil {
			logger.Log(t, fmt.Sprintf("WARNING: failure encountered during attempted k8ssandra-operator uninstall. %s", err.Error()))
		}
	}

}
