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
	"github.com/goccy/go-yaml"
	"github.com/gruntwork-io/terratest/modules/files"
	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/k8ssandra/cloud-readiness/k8ssandra/test/model"
	"github.com/k8ssandra/cloud-readiness/k8ssandra/test/util/cloud/gcp"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/clientcmd/api"
	_ "k8s.io/client-go/tools/clientcmd/api/v1"
	v1 "k8s.io/client-go/tools/clientcmd/api/v1"
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
)

func InstallK8ssandra(t *testing.T, readinessConfig model.ReadinessConfig, provisionMeta model.ProvisionMeta) {

	logger.Log(t, "\n\nK8ssandra: installations started")

	ctxOptions := preconditions(t, readinessConfig, provisionMeta)

	installControlPlaneOperator(t, readinessConfig, ctxOptions)

	installDataPlaneOperators(t, readinessConfig, ctxOptions)

	createClientConfigurations(t, readinessConfig, ctxOptions)

	installK8ssandraCluster(t, readinessConfig, ctxOptions)

}

func installDataPlaneOperators(t *testing.T, readinessConfig model.ReadinessConfig, ctxOptions map[string]model.ContextOption) {
	logger.Log(t, "\n\nK8ssandra: installation of data-plane")
	var isClusterScoped = readinessConfig.ProvisionConfig.K8cConfig.ClusterScoped
	for name, ctxConfig := range readinessConfig.Contexts {
		kubeConfig := ctxOptions[name].KubectlOptions

		setCurrentContext(t, ctxOptions[name].FullName, kubeConfig)
		isControlPlane := IsControlPlane(ctxConfig)
		kubeConfig.Env[defaultControlPlaneKey] = strconv.FormatBool(isControlPlane)

		if !isControlPlane {
			helmOptions := createHelmOptions(kubeConfig, map[string]string{
				defaultControlPlaneKey: strconv.FormatBool(isControlPlane)}, kubeConfig.Env)
			logger.Log(t, fmt.Sprintf("installing k8ssandra-operator on data-plane: %s", name))
			installOperator(t, helmOptions, name, ctxOptions[name].FullName, ctxConfig, isClusterScoped)
		}
	}
}

func installK8ssandraCluster(t *testing.T, readinessConfig model.ReadinessConfig,
	ctxOptions map[string]model.ContextOption) {

	logger.Log(t, "\n\nK8ssandra: installation of cluster")
	for name, ctxConfig := range readinessConfig.Contexts {
		kubeConfig := ctxOptions[name].KubectlOptions
		setCurrentContext(t, ctxOptions[name].FullName, kubeConfig)

		if IsControlPlane(ctxConfig) {

			// todo - address timing and re-install of this ....
			//if isK8ssandraClusterExisting(t, kubeConfig, ctxConfig.Namespace) {
			//	logger.Log(t, "k8c existing, removing")
			//	removeK8ssandraCluster(t, kubeConfig, readinessConfig.ProvisionConfig.K8cConfig.ClusterName, ctxConfig.Namespace)
			//	time.Sleep(time.Second * 15)
			//}

			kubeConfig.Env[defaultControlPlaneKey] = "true"

			logger.Log(t, fmt.Sprintf("=== deploying k8ssandra-cluster on control plane: %s", name))
			require.Eventually(t, func() bool {
				endpointIP := waitForEndpoint(t, kubeConfig, defaultK8ssandraOperatorReleaseName+"-"+defaultWebhookServiceName)
				// logger.Log(t, fmt.Sprintf("endpoint discovery on control-plane: %s", endpointIP))
				return strings.TrimSpace(endpointIP) != "''"
			}, time.Second*30, defaultInterval, "timeout waiting for endpoint ip to exist")

			logger.Log(t, "\n\nK8ssandra: control-plane k8c cluster deployment underway ...")

			deployK8ssandraCluster(t, readinessConfig, ctxConfig.Name, kubeConfig, ctxConfig.Namespace)

			time.Sleep(defaultTimeout * 6)
		}
	}
}

func deployK8ssandraCluster(t *testing.T, config model.ReadinessConfig, contextName string,
	options *k8s.KubectlOptions, namespace string) bool {
	logger.Log(t, fmt.Sprintf("deploying k8ssandra-cluster for context: [%s] namespace: [%s]",
		contextName, namespace))

	k8cConfig := config.ProvisionConfig.K8cConfig
	_, err := k8s.RunKubectlAndGetOutputE(t, options, "apply", "-f",
		path.Join("../config/", k8cConfig.ValuesFilePath), "-n", namespace)

	return err == nil
}

func restartOperator(t *testing.T, namespace string, options *k8s.KubectlOptions) {
	logger.Log(t, "\n\nK8ssandra: restarting k8ssandra-operator")

	pod, err := k8s.RunKubectlAndGetOutputE(t, options, "get", "pod",
		"-l", "app.kubernetes.io/name=k8ssandra-operator", "-n", namespace, "-o", "name")

	if err == nil && pod != "" {
		_, err := k8s.RunKubectlAndGetOutputE(t, options, "delete", "pod",
			pod, "-n", namespace)

		if err != nil {
			logger.Log(t, fmt.Sprintf("WARNING: attempt to delete pod: %s failed due to: %s", pod, err))
		}
	}

	_, err2 := k8s.RunKubectlAndGetOutputE(t, options, "rollout", "restart",
		"deployment", defaultK8ssandraOperatorReleaseName, "-n", namespace)
	require.NoError(t, err2)
	time.Sleep(defaultTimeout)
}

func restartCassOperator(t *testing.T, namespace string, options *k8s.KubectlOptions) {

	logger.Log(t, "\n\nK8ssandra: restarting k8ssandra-cass-operator")

	// k get pods -n bootz -l app.kubernetes.io/name=cass-operator -o name
	pod, err := k8s.RunKubectlAndGetOutputE(t, options, "get", "pod",
		"-l", "app.kubernetes.io/name=cass-operator", "-n", namespace, "-o", "name")

	if err == nil && pod != "" {
		_, err := k8s.RunKubectlAndGetOutputE(t, options, "delete", "pod",
			pod, "-n", namespace)

		if err != nil {
			logger.Log(t, fmt.Sprintf("WARNING: attempt to delete pod: %s failed due to: %s", pod, err))
		}
	}

	_, err2 := k8s.RunKubectlAndGetOutputE(t, options, "rollout", "restart",
		"deployment", defaultCassandraOperatorName, "-n", namespace)
	require.NoError(t, err2)
	time.Sleep(defaultTimeout)
}

func waitForEndpoint(t *testing.T, kubeConfig *k8s.KubectlOptions, name string) string {
	out, err := k8s.RunKubectlAndGetOutputE(t, kubeConfig, "get", "ep", name, "-o=jsonpath='{.subsets[0].addresses[0].ip}'")
	require.NoError(t, err, "unexpected error when attempting to obtain endpoint ip availability")
	return out
}

func installControlPlaneOperator(t *testing.T, readinessConfig model.ReadinessConfig,
	ctxOptions map[string]model.ContextOption) string {

	logger.Log(t, "\n\nK8ssandra: installing control-plane")
	var isClusterScoped = readinessConfig.ProvisionConfig.K8cConfig.ClusterScoped
	var controlPlaneContextName = ""

	for name, ctxConfig := range readinessConfig.Contexts {
		kubeConfig := ctxOptions[name].KubectlOptions
		setCurrentContext(t, ctxOptions[name].FullName, kubeConfig)

		isControlPlane := IsControlPlane(ctxConfig)
		if isControlPlane {
			setEnvErr := os.Setenv(defaultControlPlaneKey, strconv.FormatBool(isControlPlane))
			require.NoError(t, setEnvErr)

			// todo -
			// set a label on the pod -- control-plane: k8ssandra-operator

			t.Setenv(defaultControlPlaneKey, strconv.FormatBool(isControlPlane))
			kubeConfig.Env[defaultControlPlaneKey] = strconv.FormatBool(isControlPlane)
			helmOptions := createHelmOptions(kubeConfig, map[string]string{
				defaultControlPlaneKey: strconv.FormatBool(isControlPlane)}, kubeConfig.Env)

			installOperator(t, helmOptions, name, ctxOptions[name].FullName, ctxConfig, isClusterScoped)
			controlPlaneContextName = ctxOptions[name].FullName
		} else {
			setEnvErr := os.Setenv(defaultControlPlaneKey, "false")
			require.NoError(t, setEnvErr)

			t.Setenv(defaultControlPlaneKey, "false")
			kubeConfig.Env[defaultControlPlaneKey] = "false"
		}
	}
	return controlPlaneContextName
}

func createClientConfigurations(t *testing.T, readinessConfig model.ReadinessConfig,
	ctxOptions map[string]model.ContextOption) {
	logger.Log(t, "\n\nK8ssandra: creating client configurations")

	var generatedClientConfigs []string
	var kubeConfig *k8s.KubectlOptions

	for name, ctxConfig := range readinessConfig.Contexts {

		kubeConfig = ctxOptions[name].KubectlOptions
		setCurrentContext(t, ctxOptions[name].FullName, kubeConfig)

		addServiceAccount(t, ctxOptions[name], ctxConfig.Namespace, kubeConfig)
		setupTestArtifactDirectory(t, ctxOptions[name])

		generatedClientConfig := generateClientConfig(t, ctxOptions[name])
		generatedClientConfigs = append(generatedClientConfigs, generatedClientConfig)
	}

	createConfigs(t, ctxOptions, readinessConfig)

	logger.Log(t, "\n\nK8ssandra: Creating the generic secret ...")
	for name, ctxConfig := range readinessConfig.Contexts {
		kubeConfig = ctxOptions[name].KubectlOptions
		createGenericSecret(t, ctxConfig.Namespace, kubeConfig)
	}

	// Apply generated client configs for each cluster.
	for name, ctxConfig := range readinessConfig.Contexts {
		for _, gcc := range generatedClientConfigs {
			setCurrentContext(t, ctxOptions[name].FullName, ctxOptions[name].KubectlOptions)
			applyClientConfig(t, ctxOptions[name].KubectlOptions, gcc, ctxConfig.Namespace)
		}
	}

	// delete pods, then perform a rollout restart of the operators.
	for name, ctxConfig := range readinessConfig.Contexts {
		restartOperator(t, ctxConfig.Namespace, ctxOptions[name].KubectlOptions)
		restartCassOperator(t, ctxConfig.Namespace, ctxOptions[name].KubectlOptions)
	}
}

func setupTestArtifactDirectory(t *testing.T, ctxOption model.ContextOption) {

	rootPath := configRootPath(t, ctxOption, "")
	mkdError := os.MkdirAll(rootPath, defaultTempFilePerm)
	require.NoError(t, mkdError, fmt.Sprintf("Unable to setup tmp file location for test artifacts"+
		"root path: %s", rootPath))
}

func createConfigs(t *testing.T, ctxOptions map[string]model.ContextOption, readinessConfig model.ReadinessConfig) {

	var clusters []v1.NamedCluster
	var auths []v1.NamedAuthInfo
	var namedContexts []v1.NamedContext
	var currentContext string

	for name := range readinessConfig.Contexts {
		ctxOption := ctxOptions[name]

		var cluster = v1.Cluster{
			Server:                   ctxOption.ServerAddress,
			InsecureSkipTLSVerify:    false,
			CertificateAuthorityData: ctxOption.ServiceAccount.Cert,
		}

		var namedCluster = v1.NamedCluster{
			Name:    ctxOption.FullName,
			Cluster: cluster,
		}
		clusters = append(clusters, namedCluster)

		var authInfo = v1.AuthInfo{
			Token: ctxOption.ServiceAccount.Token,
		}

		userName := ctxOption.FullName
		var namedAuthInfo = v1.NamedAuthInfo{
			Name:     userName,
			AuthInfo: authInfo,
		}
		auths = append(auths, namedAuthInfo)

		var context = v1.Context{
			Cluster:  ctxOption.FullName,
			AuthInfo: namedAuthInfo.Name,
		}

		if IsControlPlane(readinessConfig.Contexts[name]) {
			currentContext = ctxOption.FullName
		}

		var namedContext = v1.NamedContext{
			Name:    userName,
			Context: context,
		}
		namedContexts = append(namedContexts, namedContext)
	}

	var cfg = v1.Config{
		Kind:           "Config",
		APIVersion:     "v1",
		Preferences:    v1.Preferences{},
		Clusters:       clusters,
		AuthInfos:      auths,
		Contexts:       namedContexts,
		CurrentContext: currentContext,
		Extensions:     nil,
	}

	for name := range ctxOptions {
		ctxOption := ctxOptions[name]
		absolutePath := writeKubeConfig(t, ctxOption, cfg)
		kubeConfig := k8s.NewKubectlOptions(ctxOption.FullName, absolutePath, ctxOption.ServiceAccount.Namespace)

		*(ctxOptions[name].KubectlOptions) = *kubeConfig
		logger.Log(t, fmt.Sprintf("Assigned ctx name: %s to kube config ctx: %s @ %s", name, kubeConfig.ContextName, kubeConfig.ConfigPath))
	}

}

func installOperator(t *testing.T, helmOptions *helm.Options, name string, fullName string,
	ctxConfig model.ContextConfig, isClusterScoped bool) {

	logger.Log(t, fmt.Sprintf("===installing operator name: %s and full-name: %s", name, fullName))
	installK8ssandraOperator(t, helmOptions, ctxConfig.Name, ctxConfig.Namespace, isClusterScoped)
}

func createIdentityEnv(configPath string, identity string, credPath string) map[string]string {
	return map[string]string{
		"KUBECONFIG":                     configPath,
		"GOOGLE_IDENTITY_EMAIL":          identity,
		"GOOGLE_APPLICATION_CREDENTIALS": credPath,
	}
}

func setCurrentContext(t *testing.T, ctxName string, kubeConfig *k8s.KubectlOptions) bool {
	kubeConfig.Env["KUBECONFIG"] = kubeConfig.ConfigPath
	logger.Log(t, fmt.Sprintf("==== setting current context with kubeconfig target: %s", kubeConfig.Env["KUBECONFIG"]))
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
	logger.Log(t, fmt.Sprintf("cloud temp context-specific root path: %s", rootPath))
	return rootPath
}

func configCloudTempRootPath(t *testing.T, contextOption model.ContextOption, fileName string) string {
	rootPath := contextOption.ProvisionMeta.ArtifactsRootDir
	if fileName != "" {
		return path.Join(rootPath, fileName)
	}
	logger.Log(t, fmt.Sprintf("cloud temp root path: %s", rootPath))
	return rootPath
}

func createGenericSecret(t *testing.T, namespace string, kubeConfig *k8s.KubectlOptions) {
	logger.Log(t, fmt.Sprintf("generating secret with name: %s", defaultK8ssandraSecret))

	kubeConfig.Namespace = namespace
	var _, err = k8s.RunKubectlAndGetOutputE(t, kubeConfig, "create", "secret", "generic",
		defaultK8ssandraSecret, "-n", namespace, "--from-file", kubeConfig.ConfigPath)

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

func generateClientConfig(t *testing.T, ctxOption model.ContextOption) string {
	var clientConfigSpec = model.ClientConfigSpec{
		ContextName:      ctxOption.FullName,
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

	logger.Log(t, fmt.Sprintf("writing client-config to: %s ", absoluteFilePath))
	writeError := ioutil.WriteFile(absoluteFilePath, yamlOut, defaultTempFilePerm)
	require.NoError(t, writeError, fmt.Sprintf("Unable to write client-config: %s", absoluteFilePath))

	return absoluteFilePath
}

func writeKubeConfig(t *testing.T, ctxOption model.ContextOption, clientConfig v1.Config) string {
	yamlOut, marshalError := yaml.Marshal(&clientConfig)
	if marshalError != nil {
		logger.Log(t, marshalError.Error())
	}

	fileName := defaultKubeConfigFileName
	absoluteFilePath := configCloudTempRootPath(t, ctxOption, fileName)

	if files.FileExists(absoluteFilePath) {
		err := os.Remove(absoluteFilePath)
		require.NoError(t, err, fmt.Sprintf("Unable to cleanup existing kube-config: %s", absoluteFilePath))
	}

	mkdError := os.MkdirAll(configRootPath(t, ctxOption, ""), defaultTempFilePerm)
	require.NoError(t, mkdError, fmt.Sprintf("Unable to setup tmp file location for kube config artifact "+
		"to: %s", absoluteFilePath))
	logger.Log(t, fmt.Sprintf("writing kube config to: %s ", absoluteFilePath))
	writeError := ioutil.WriteFile(absoluteFilePath, yamlOut, defaultTempFilePerm)
	require.NoError(t, writeError, fmt.Sprintf("Unable to write kube config: %s", absoluteFilePath))
	return absoluteFilePath
}

func addServiceAccount(t *testing.T, ctxOption model.ContextOption, namespace string,
	kubeConfig *k8s.KubectlOptions) {

	logger.Log(t, fmt.Sprintf("adding service account:%s to context using ns:%s", defaultK8ssandraOperatorReleaseName, namespace))

	kubeConfig.Namespace = namespace
	csa := model.ContextServiceAccount{}
	csa.Namespace = namespace

	csa.Secret = FetchSecret(t, kubeConfig, defaultK8ssandraOperatorReleaseName, namespace)
	csa.Token = FetchToken(t, kubeConfig, csa.Secret, namespace)
	csa.Cert, _ = FetchCertificate(t, kubeConfig, csa.Secret, namespace)

	require.NotEmpty(t, csa.Cert, "Expected certificate data available for secret")
	*ctxOption.ServiceAccount = csa

	logger.Log(t, fmt.Sprintf("certificate and token obtained for secret:%s", csa.Secret))
}

func installCertManager(t *testing.T, options *k8s.KubectlOptions) {
	withoutNamespace := &options

	// Necessary as the cert manager configuration currently used, specifies its own namespaces
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

func installK8ssandraOperator(t *testing.T, options *helm.Options, contextName string, namespace string, isClusterScoped bool) {
	logger.Log(t, fmt.Sprintf("installing k8ssandra-operator "+
		"for context: %s and namespace: %s", contextName, namespace))
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

func preconditions(t *testing.T, readinessConfig model.ReadinessConfig,
	provisionMeta model.ProvisionMeta) map[string]model.ContextOption {

	identity := FetchEnv(t, provisionMeta.AdminIdentity)
	require.NotEmpty(t, identity, "expecting identity to be provided to apply preconditions")

	env := createIdentityEnv(provisionMeta.DefaultConfigPath, identity, readinessConfig.ProvisionConfig.CloudConfig.CredPath)

	// TODO - move gcp usage to generic (cloud-agnostic) util
	gcp.Switch(t, identity, env)

	var contextConfigs = map[string]*k8s.KubectlOptions{}
	var isRepoSetup = false
	for name := range readinessConfig.Contexts {
		logger.Log(t, fmt.Sprintf("precondition for name: %s", name))
		fullName := gcp.ConstructFullContextName(name, readinessConfig)

		// TODO - move gcp usage to generic (cloud-agnostic) util
		gcp.FetchCreds(t, readinessConfig, env, gcp.ConstructCloudClusterName(name,
			readinessConfig.ProvisionConfig.CloudConfig))

		kubeConfig := k8s.NewKubectlOptions(fullName, provisionMeta.DefaultConfigPath,
			readinessConfig.Contexts[name].Namespace)

		setCurrentContext(t, fullName, kubeConfig)

		helmOptions := createHelmOptions(kubeConfig, map[string]string{}, map[string]string{})
		if !isRepoSetup {
			isRepoSetup = repoSetup(t, helmOptions)
		}

		installCertManager(t, kubeConfig)

		contextConfigs[name] = kubeConfig

		installTraefik(t, helmOptions, readinessConfig.Contexts[name])
	}

	return createContextOptions(t, readinessConfig, provisionMeta, contextConfigs)
}

func createContextOptions(t *testing.T, readinessConfig model.ReadinessConfig,
	provisionMeta model.ProvisionMeta, configs map[string]*k8s.KubectlOptions) map[string]model.ContextOption {

	logger.Log(t, fmt.Sprintf("\n\ncreating all context options for "+
		"provision id: %s", provisionMeta.ProvisionId))

	ctxOptions := map[string]model.ContextOption{}

	for name, ctx := range readinessConfig.Contexts {
		fullName := gcp.ConstructFullContextName(name, readinessConfig)
		logger.Log(t, fmt.Sprintf("creating context options for context:%s with ns:%s",
			ctx.Name, ctx.Namespace))

		cloudClusterName := gcp.ConstructCloudClusterName(name, readinessConfig.ProvisionConfig.CloudConfig)
		saName := cloudClusterName + "-" + readinessConfig.ServiceAccountNameSuffix + defaultIdentityDomain

		kubeCluster := selectClusterFromKube(t, name, configs)
		require.NotNil(t, kubeCluster, fmt.Sprintf("expected kube cluster to be found for name: %s", name))

		logger.Log(t, fmt.Sprintf("setting context options with service account: %s and "+
			"server: %s", saName, kubeCluster.Server))
		ctxOptions[name] = model.ContextOption{
			ShortName:      name,
			FullName:       fullName,
			KubectlOptions: configs[name],
			AdminOptions:   configs[name],
			ServiceAccount: &model.ContextServiceAccount{Name: saName, Namespace: ctx.Namespace,
				Cert: kubeCluster.CertificateAuthorityData},
			ServerAddress: kubeCluster.Server,
			ProvisionMeta: provisionMeta,
		}
	}
	return ctxOptions
}

func selectClusterFromKube(t *testing.T, name string, configs map[string]*k8s.KubectlOptions) *api.Cluster {

	ko := configs[name]
	rawConfig, err := k8s.LoadConfigFromPath(ko.ConfigPath).RawConfig()
	require.NoError(t, err, "Expecting to be able to obtain infrastructure provisioned cluster raw configuration")
	for clusterName, config := range rawConfig.Clusters {
		if strings.Contains(clusterName, name) {
			return config
		}
	}
	return nil
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

func installTraefik(t *testing.T, helmOptions *helm.Options, config model.ContextConfig) {

	require.NotNil(t, helmOptions, "expecting helm options to install traefik")
	require.NotNil(t, config, "expecting readiness config to install traefik")

	DeleteResource(t, helmOptions.KubectlOptions, "ClusterRoleBinding", defaultTraefikResourceName)
	DeleteResource(t, helmOptions.KubectlOptions, "ClusterRole", defaultTraefikResourceName)

	uninstallTraefik(t, helmOptions)

	_, err := helm.RunHelmCommandAndGetStdOutE(t, helmOptions, "install", defaultTraefikRepositoryName,
		defaultTraefikChartName, "--version", config.NetworkConfig.TraefikVersion, "-f",
		path.Join("../config/", config.NetworkConfig.TraefikValuesFile))

	require.NoError(t, err, "expecting that traefik can be installed")
}

func uninstallTraefik(t *testing.T, helmOptions *helm.Options) {

	_, err := helm.RunHelmCommandAndGetStdOutE(t, helmOptions, "uninstall", defaultTraefikRepositoryName)
	if err != nil {
		logger.Log(t, fmt.Sprintf("WARNING: failure encountered during attempted traefik uninstall. %s", err.Error()))
	}
}
