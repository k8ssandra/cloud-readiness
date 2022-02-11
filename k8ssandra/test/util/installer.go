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
	"github.com/gruntwork-io/terratest/modules/files"
	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/k8ssandra/cloud-readiness/k8ssandra/test/model"
	"github.com/k8ssandra/cloud-readiness/k8ssandra/test/util/cloud/gcp"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	_ "gopkg.in/yaml.v3"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	_ "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/utils/strings/slices"
	"os"
	"path"
	"strconv"
	"strings"
	"testing"
)

const (
	defaultTempFilePerm            = os.FileMode(0700)
	defaultK8ssandraSecret         = "k8s-contexts"
	defaultIdentityDomain          = "@community-ecosystem.iam.gserviceaccount.com"
	defaultK8ssandraServiceAccount = "k8ssandra-operator"
)

func InstallK8ssandra(t *testing.T, readinessConfig model.ReadinessConfig, provisionMeta model.ProvisionMeta) {
	logger.Log(t, "installation started for k8ssandra")
	preconditions(t, readinessConfig, provisionMeta)

	ctxOptions := createContextOptions(t, readinessConfig, provisionMeta)
	controlPlaneContextName := installControlPlane(t, readinessConfig, ctxOptions)

	installDataPlanes(t, readinessConfig, ctxOptions)
	installK8ssandraCluster(t, readinessConfig, ctxOptions)

	createClientConfigurations(t, readinessConfig, ctxOptions, controlPlaneContextName)
}

func installDataPlanes(t *testing.T, readinessConfig model.ReadinessConfig, ctxOptions map[string]model.ContextOption) {
	logger.Log(t, "\n\nINSTALLING K8SSANDRA DATA PLANES")
	var isClusterScoped = readinessConfig.ProvisionConfig.K8cConfig.ClusterScoped
	for name, ctxConfig := range readinessConfig.Contexts {
		kubeConfig := ctxOptions[name].KubectlOptions
		setCurrentContext(t, ctxOptions[name].FullName, kubeConfig)
		isControlPlane := slices.Contains(ctxConfig.ClusterLabels, "control-plane")
		envs := map[string]string{"K8SSANDRA_CONTROL_PLANE": strconv.FormatBool(isControlPlane)}
		kubeConfig.Env = envs
		if !isControlPlane {
			helmOptions := createHelmOptions(kubeConfig, map[string]string{
				"K8SSANDRA_CONTROL_PLANE": strconv.FormatBool(isControlPlane)}, envs)
			logger.Log(t, fmt.Sprintf("installing k8ssandra-operator on data-plane: %s", name))
			installOperator(t, helmOptions, name, ctxOptions[name].FullName, ctxConfig, isClusterScoped)
		}
	}
}

func installK8ssandraCluster(t *testing.T, readinessConfig model.ReadinessConfig, ctxOptions map[string]model.ContextOption) {
	logger.Log(t, "\n\nINSTALLING K8SSANDRA DATA PLANES")
	for name, ctxConfig := range readinessConfig.Contexts {
		kubeConfig := ctxOptions[name].KubectlOptions
		setCurrentContext(t, ctxOptions[name].FullName, kubeConfig)
		if slices.Contains(ctxConfig.ClusterLabels, "control-plane") {
			envs := map[string]string{"K8SSANDRA_CONTROL_PLANE": "true"}
			kubeConfig.Env = envs
			logger.Log(t, fmt.Sprintf("deploying k8ssandra-cluster on control plane: %s", name))
			deployK8ssandraCluster(t, readinessConfig, ctxConfig.Name, kubeConfig, ctxConfig.Namespace)
		}
	}
}

func installControlPlane(t *testing.T, readinessConfig model.ReadinessConfig,
	ctxOptions map[string]model.ContextOption) string {
	logger.Log(t, "\n\nINSTALLING K8SSANDRA CONTROL-PLANE")

	var isClusterScoped = readinessConfig.ProvisionConfig.K8cConfig.ClusterScoped
	var controlPlaneContextName = ""

	for name, ctxConfig := range readinessConfig.Contexts {
		kubeConfig := ctxOptions[name].KubectlOptions
		setCurrentContext(t, ctxOptions[name].FullName, kubeConfig)
		isControlPlane := slices.Contains(ctxConfig.ClusterLabels, "control-plane")
		if isControlPlane {
			envs := map[string]string{"K8SSANDRA_CONTROL_PLANE": strconv.FormatBool(isControlPlane)}
			kubeConfig.Env = envs
			helmOptions := createHelmOptions(kubeConfig, map[string]string{
				"K8SSANDRA_CONTROL_PLANE": strconv.FormatBool(isControlPlane)}, envs)
			installOperator(t, helmOptions, name, ctxOptions[name].FullName, ctxConfig, isClusterScoped)
			controlPlaneContextName = ctxOptions[name].FullName
		}
	}
	return controlPlaneContextName
}

func createClientConfigurations(t *testing.T, readinessConfig model.ReadinessConfig,
	ctxOptions map[string]model.ContextOption, controlPlaneContextName string) {

	logger.Log(t, "\n\nCREATING CLIENT CONFIGURATIONS")
	for name, ctxConfig := range readinessConfig.Contexts {
		kubeConfig := ctxOptions[name].KubectlOptions
		setCurrentContext(t, ctxOptions[name].FullName, kubeConfig)

		namedContextOption := ctxOptions[name]
		addServiceAccount(t, &namedContextOption, ctxConfig.Namespace, kubeConfig)

		clientConfig := ExternalizeConfig(t, namedContextOption, k8s.LoadConfigFromPath(kubeConfig.ConfigPath))
		setCurrentContext(t, controlPlaneContextName, kubeConfig)

		createGenericSecret(t, ctxConfig.Namespace, kubeConfig)
		generatedClientConfig := generateClientConfig(t, clientConfig, namedContextOption)
		applyClientConfig(t, kubeConfig, generatedClientConfig, ctxConfig.Namespace)
	}
}

func installOperator(t *testing.T, helmOptions *helm.Options, name string, fullName string,
	ctxConfig model.ContextConfig, isClusterScoped bool) {

	logger.Log(t, fmt.Sprintf("installing operator name: %s and full-name: %s", name, fullName))
	installK8ssandraOperator(t, helmOptions, ctxConfig.Name, ctxConfig.Namespace, isClusterScoped)
}

func auth(t *testing.T, clusterName string, ctxOption model.ContextOption, readinessConfig model.ReadinessConfig,
	meta model.ProvisionMeta, kubeConfig *k8s.KubectlOptions, isAdminAuth bool) bool {
	require.NotEmpty(t, meta.ArtifactsRootDir, "expecting artifacts root directory to be defined")

	var identity = ""
	if isAdminAuth {
		identity = FetchEnv(t, meta.AdminIdentity)
	} else {
		identity = clusterName
	}

	env := map[string]string{
		"KUBECONFIG":                     kubeConfig.ConfigPath,
		"GOOGLE_IDENTITY_EMAIL":          identity,
		"GOOGLE_APPLICATION_CREDENTIALS": readinessConfig.ProvisionConfig.CloudConfig.CredPath,
	}
	setCurrentContext(t, ctxOption.FullName, kubeConfig)

	if isAdminAuth {
		return gcp.Switch(t, identity, env)
	} else {
		if !files.IsExistingDir(meta.ArtifactsRootDir) {
			err := os.MkdirAll(meta.ArtifactsRootDir, defaultTempFilePerm)
			if err != nil {
				require.NoError(t, err, "activation of service account(s) requires user to have "+
					"appropriate permissions")
				return false
			}
		}
		if gcp.ActivateServiceAccount(t, env, meta.ArtifactsRootDir, identity) {
			logger.Log(t, fmt.Sprintf("activated service account for identity:%s for cluster:%s ",
				identity, clusterName))
			return gcp.FetchCreds(t, readinessConfig, env, clusterName)
		} else {
			logger.Log(t, fmt.Sprintf("unable to activate service account for identity:%s ", identity))
		}
	}
	return false
}

func setCurrentContext(t *testing.T, ctxName string, kubeConfig *k8s.KubectlOptions) bool {
	_, err := k8s.RunKubectlAndGetOutputE(t, kubeConfig, "config", "set", "current-context", ctxName)
	require.NoError(t, err, "expecting to set current context without error")
	return err == nil
}

func applyClientConfig(t *testing.T, options *k8s.KubectlOptions, clientConfigFile string, namespace string) {
	_, err := k8s.RunKubectlAndGetOutputE(t, options, "-n", namespace, "apply", "-f", clientConfigFile)
	require.NoError(t, err)
}

func configRootPath(t *testing.T, contextOption model.ContextOption, fileName string) string {
	rootPath := path.Join(contextOption.ProvisionMeta.ArtifactsRootDir, contextOption.FullName)
	if fileName != "" {
		return path.Join(rootPath, fileName)
	}
	logger.Log(t, fmt.Sprintf("config root path: %s", rootPath))
	return rootPath
}

func createGenericSecret(t *testing.T, namespace string, kubeConfig *k8s.KubectlOptions) {
	logger.Log(t, fmt.Sprintf("generating secret with name: %s", defaultK8ssandraSecret))

	var _, err = k8s.RunKubectlAndGetOutputE(t, kubeConfig,
		"create", "secret", "generic", defaultK8ssandraSecret, "-n", namespace, "--from-file", kubeConfig.ConfigPath)

	if err != nil {
		// Try recovery by removing existing.
		_, err2 := k8s.RunKubectlAndGetOutputE(t, kubeConfig, "delete", "secret", defaultK8ssandraSecret,
			"-n", namespace)
		require.NoError(t, err2)

		_, err = k8s.RunKubectlAndGetOutputE(t, kubeConfig,
			"create", "secret", "generic", defaultK8ssandraSecret, "-n", namespace, "--from-file", kubeConfig.ConfigPath)
	}
	require.NoError(t, err)
}

func generateClientConfig(t *testing.T, config clientcmd.ClientConfig, ctxOption model.ContextOption) string {
	rawConfig, _ := config.RawConfig()
	var clientConfigSpec = model.ClientConfigSpec{
		ContextName:      rawConfig.CurrentContext,
		KubeConfigSecret: corev1.LocalObjectReference{Name: defaultK8ssandraSecret},
	}

	clientConfigName := strings.ReplaceAll(ctxOption.FullName, "_", "-")
	objectMeta := model.ObjectMeta{Name: clientConfigName}

	clientConfig := model.ClientConfig{
		ApiVersion: "config.k8ssandra.io/v1beta1",
		Spec:       clientConfigSpec,
		Kind:       "ClientConfig",
		Metadata:   objectMeta,
	}
	return writeClientConfig(t, ctxOption, clientConfig)
}

func writeClientConfig(t *testing.T, ctxOption model.ContextOption, clientConfig model.ClientConfig) string {
	yamlOut, marshalError := yaml.Marshal(&clientConfig)
	if marshalError != nil {
		logger.Log(t, marshalError.Error())
	}

	fileName := ctxOption.ShortName + "-client_config.yaml"
	absoluteFilePath := configRootPath(t, ctxOption, fileName)

	if files.FileExists(absoluteFilePath) {
		err := os.Remove(absoluteFilePath)
		require.NoError(t, err, fmt.Sprintf("Unable to cleanup existing client-config: %s", absoluteFilePath))
	}

	mkdError := os.MkdirAll(configRootPath(t, ctxOption, ""), defaultTempFilePerm)
	require.NoError(t, mkdError, fmt.Sprintf("Unable to setup tmp file location for test artifact client-config to: %s", absoluteFilePath))

	writeError := ioutil.WriteFile(absoluteFilePath, yamlOut, defaultTempFilePerm)
	require.NoError(t, writeError, fmt.Sprintf("Unable to write client-config: %s", absoluteFilePath))
	return absoluteFilePath
}

func addServiceAccount(t *testing.T, namedCtxOption *model.ContextOption, namespace string, kubeConfig *k8s.KubectlOptions) {
	logger.Log(t, fmt.Sprintf("Adding service account to context using ns:%s", namespace))
	csa := model.ContextServiceAccount{}
	csa.Namespace = namespace
	csa.Secret = FetchSecret(t, kubeConfig, defaultK8ssandraServiceAccount, namespace)
	csa.Token = FetchToken(t, kubeConfig, csa.Secret, namespace)
	csa.Cert = FetchCertificate(t, kubeConfig, csa.Secret, namespace)
	namedCtxOption.ServiceAccount = csa
}

func installCertManager(t *testing.T, options *k8s.KubectlOptions) {
	withoutNamespace := &options
	// Necessary as the cert manager configuration currently used, specifies its own namespaces
	(*withoutNamespace).Namespace = ""
	(*withoutNamespace).Env = map[string]string{"installCRDs": "true"}

	_, err := k8s.RunKubectlAndGetOutputE(t, *withoutNamespace,
		"apply", "-f", defaultCertManagerFile)

	// Retry
	if err != nil {
		logger.Log(t, "retrying install cert manager ...")
		_, err2 := k8s.RunKubectlAndGetOutputE(t, *withoutNamespace,
			"apply", "-f", defaultCertManagerFile)
		require.NoError(t, err2)
	}
}

func installK8ssandraOperator(t *testing.T, options *helm.Options, contextName string, namespace string, isClusterScoped bool) {
	logger.Log(t, fmt.Sprintf("installing [k8ssandra-operator] "+
		"for context: [%s] and namespace: [%s]", contextName, namespace))
	logger.Log(t, fmt.Sprintf("cluster scoped for k8ssandra-operator is set as: %s",
		strconv.FormatBool(isClusterScoped)))

	result, err := helm.RunHelmCommandAndGetOutputE(t, options, "install",
		defaultK8ssandraOperatorReleaseName, defaultK8ssandraOperatorChart, "-n", namespace,
		"--create-namespace")

	if err != nil {
		logger.Log(t, fmt.Sprintf("failed k8ssandra install due to error: %s", err.Error()))
	} else {
		logger.Log(t, fmt.Sprintf("installation result: %s", result))
	}
}

func preconditions(t *testing.T, readinessConfig model.ReadinessConfig, provisionMeta model.ProvisionMeta) {
	identity := FetchEnv(t, provisionMeta.AdminIdentity)
	require.NotEmpty(t, identity, "expecting identity to be provided to apply preconditions")

	env := map[string]string{
		"KUBECONFIG":                     provisionMeta.DefaultConfigPath,
		"GOOGLE_IDENTITY_EMAIL":          identity,
		"GOOGLE_APPLICATION_CREDENTIALS": readinessConfig.ProvisionConfig.CloudConfig.CredPath,
	}
	gcp.Switch(t, identity, env)

	var isRepoSetup = false
	for name := range readinessConfig.Contexts {
		logger.Log(t, fmt.Sprintf("precondition for name: %s", name))
		fullName := gcp.ConstructFullContextName(name, readinessConfig)

		gcp.FetchCreds(t, readinessConfig, env, gcp.ConstructCloudClusterName(name, readinessConfig.ProvisionConfig.CloudConfig))
		kubeConfig := k8s.NewKubectlOptions(fullName, provisionMeta.DefaultConfigPath, "")
		setCurrentContext(t, fullName, kubeConfig)

		helmOptions := createHelmOptions(kubeConfig, map[string]string{}, map[string]string{})
		if !isRepoSetup {
			isRepoSetup = repoSetup(t, helmOptions)
		}
		installCertManager(t, kubeConfig)
	}
}

// createContextOptions builds a map of options specific to a context
func createContextOptions(t *testing.T, readinessConfig model.ReadinessConfig,
	provisionMeta model.ProvisionMeta) map[string]model.ContextOption {
	ctxOptions := map[string]model.ContextOption{}
	logger.Log(t, fmt.Sprintf("\n\nobtaining kube configs for all contexts related to provision id: %s", provisionMeta.ProvisionId))

	for name, ctx := range readinessConfig.Contexts {
		fullName := gcp.ConstructFullContextName(name, readinessConfig)
		logger.Log(t, fmt.Sprintf("creating context options for context:%s with ns:%s", ctx.Name, ctx.Namespace))
		kubeConfig := k8s.NewKubectlOptions(fullName, provisionMeta.DefaultConfigPath, ctx.Namespace)
		saName := gcp.ConstructCloudClusterName(name, readinessConfig.ProvisionConfig.CloudConfig) + "-" +
			readinessConfig.ServiceAccountNameSuffix + defaultIdentityDomain
		logger.Log(t, fmt.Sprintf("setting context options with service account: %s", saName))
		ctxOptions[name] = model.ContextOption{
			ShortName:      name,
			FullName:       fullName,
			KubectlOptions: kubeConfig,
			ServiceAccount: model.ContextServiceAccount{Name: saName, Namespace: ctx.Namespace},
			ServerAddress:  "",
			ProvisionMeta:  provisionMeta,
		}
		auth(t, saName, ctxOptions[name], readinessConfig, provisionMeta, kubeConfig, false)
	}
	return ctxOptions
}
