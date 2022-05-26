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
	"github.com/goccy/go-yaml"
	"github.com/gruntwork-io/terratest/modules/files"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/k8ssandra/cloud-readiness/k8ssandra/test/model"
	"github.com/k8ssandra/cloud-readiness/k8ssandra/test/util/cloud/gcp"
	"github.com/mitchellh/go-homedir"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/clientcmd/api"
	v1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/utils/strings/slices"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	defaultArtifactFormat       = "/tmp/TestK8cSmoke(\\w+)/"
	defaultParentArtifactFormat = "/tmp/(\\w+)"
)

// Apply based on provision meta and configuration settings
func Apply(t *testing.T, meta model.ProvisionMeta, readinessConfig model.ReadinessConfig) {

	logger.Log(t, fmt.Sprintf("SIMULATE mode: %s", strconv.FormatBool(meta.Enable.Simulate)))
	if meta.Enable.RemoveAll {

		logger.Log(t, fmt.Sprintf("remove all requested, existing infrastructure provisioning "+
			"is being referenced: %s. Starting artifact removal.", meta.ArtifactsRootDir))
		RemoveProvisioningArtifacts(t, meta, readinessConfig, true)

	} else if meta.Enable.ProvisionInfra && !meta.Enable.Install {
		logger.Log(t, fmt.Sprintf("existing infrastructure provisioning is not being referenced, "+
			"provision started %s", meta.ProvisionId))
		meta = ProvisionMultiCluster(t, readinessConfig, meta)
		require.NotEmpty(t, meta.ProvisionId, "expected provision step to occur.")

	} else if meta.Enable.Install && !meta.Enable.ProvisionInfra {
		logger.Log(t, fmt.Sprintf("installation starting for provision identifier: %s", meta.ProvisionId))
		InstallK8ssandra(t, readinessConfig, meta)
	} else if meta.Enable.PreInstallSetup && !meta.Enable.ProvisionInfra {
		PreInstallSetup(t, meta, readinessConfig)
	} else {
		logger.Log(t, fmt.Sprintf("NOTICE: a single meta activity is not provided for apply "+
			"(e.g. Install, ProvisionInfra, RemoveAll).  It may be required that another enablement is causing conflict."))
	}

}

func CreateTerraformOptions(meta model.ProvisionMeta, config model.ReadinessConfig,
	name string, ctx model.ContextConfig, kubeConfigPath string, rootFolder string) terraform.Options {

	uniqueClusterName := strings.ToLower(fmt.Sprintf(name))
	saName := gcp.ConstructCloudClusterName(name, ctx.CloudConfig) + "-" +
		config.ServiceAccountNameSuffix + defaultIdentityDomain

	nodePools := createNodePools(ctx)

	uniqueBucketName := strings.ToLower(fmt.Sprintf(ctx.CloudConfig.Bucket+"-%s", config.UniqueId))
	vars := map[string]interface{}{
		"project_id":              ctx.CloudConfig.Project,
		"name":                    uniqueClusterName,
		"machine_type":            ctx.CloudConfig.MachineType,
		"environment":             ctx.CloudConfig.Environment,
		"provision_id":            meta.ProvisionId,
		"region":                  ctx.CloudConfig.Region,
		"zone":                    ctx.CloudConfig.Region,
		"node_pools":              nodePools,
		"node_locations":          ctx.CloudConfig.Locations,
		"kubectl_config_path":     kubeConfigPath,
		"initial_node_count":      config.ExpectedNodeCount,
		"cluster_name":            uniqueClusterName,
		"service_account":         saName,
		"enable_private_endpoint": false,
		"enable_private_nodes":    false,
		"cidr_block":              ctx.NetworkConfig.SubnetCidrBlock,
		"secondary_cidr_block":    ctx.NetworkConfig.SecondaryCidrBlock,
		"master_ipv4_cidr_block":  ctx.NetworkConfig.MasterIpv4CidrBlock,
		"bucket_policy_only":      true,
		"role":                    "roles/storage.admin",
		ctx.CloudConfig.Bucket:    uniqueBucketName,
	}

	if meta.Enable.Simulate {
		println("SIMULATE: tf options output:")
		for k, v := range vars {
			println(fmt.Sprintf(" [%s,%s]", k, v))
		}
	}

	envVars := map[string]string{"GOOGLE_APPLICATION_CREDENTIALS": ctx.CloudConfig.CredPath,
		defaultControlPlaneKey: strconv.FormatBool(IsControlPlane(config.Contexts[name]))}

	return terraform.Options{
		TerraformDir: rootFolder,
		Vars:         vars,
		EnvVars:      envVars,
	}

}

func createNodePools(ctx model.ContextConfig) []map[string]interface{} {

	var nodePools []map[string]interface{}
	for _, prc := range ctx.CloudConfig.PoolRackConfigs {
		var nodePool = map[string]interface{}{}
		nodePool["label"] = prc.Label
		nodePool["name"] = prc.Name
		nodePool["location"] = prc.Location
		nodePools = append(nodePools, nodePool)
	}
	return nodePools
}

func IsControlPlane(ctxConfig model.ContextConfig) bool {
	return slices.Contains(ctxConfig.ClusterLabels, defaultControlPlaneLabel)
}

func FetchCertificate(t *testing.T, options *k8s.KubectlOptions, secret string, namespace string) ([]byte, error) {
	logger.Log(t, fmt.Sprintf("obtaining certificate"))
	out, err := k8s.RunKubectlAndGetOutputE(t, options, "get", "secret", secret, "-n", namespace, "-o", "jsonpath={.data['ca\\.crt']}")
	require.NoError(t, err)
	return base64.StdEncoding.DecodeString(out)
}

func FetchToken(t *testing.T, options *k8s.KubectlOptions, secret string, namespace string) string {
	out, err := k8s.RunKubectlAndGetOutputE(t, options, "--context", options.ContextName,
		"-n", namespace, "get", "secret", secret, "-o", "jsonpath={.data.token}")

	require.NoError(t, err)
	require.NotNil(t, out)

	decoded, err := base64.StdEncoding.DecodeString(out)
	if err != nil {
		log.Fatalf("Some error occured during base64 decode. Error %s", err.Error())
	}
	return string(decoded)
}

func FetchSecret(t *testing.T, options *k8s.KubectlOptions, serviceAccount string, namespace string) string {

	options.Namespace = namespace
	sa := k8s.GetServiceAccount(t, options, serviceAccount)
	require.NotNil(t, sa, fmt.Sprintf("Expecting service account to be available: %s", serviceAccount))
	secret := sa.Secrets[0].Name
	require.NotNil(t, secret, fmt.Sprintf("Expecting secret to be availabe for service account: %s", serviceAccount))
	return secret
}

func FetchKubeConfigPath(t *testing.T) (string, string) {
	home, err := homedir.Dir()
	require.NoError(t, err, "unable to locate home directory for config path")
	return home, filepath.Join(home, ".kube", "kubeconfig")
}

func FetchEnv(t *testing.T, key string) string {
	require.NotEmpty(t, key, "expecting key to be defined for fetch env")
	return os.Getenv(key)
}

func CreateClientConfigurations(t *testing.T, meta model.ProvisionMeta, readinessConfig model.ReadinessConfig,
	ctxOptions map[string]model.ContextOption) {

	if meta.Enable.Simulate {
		logger.Log(t, "\n\nK8ssandra: SIMULATE creating client configurations")
		return
	}

	logger.Log(t, "\n\nK8ssandra: creating client configurations")
	var generatedClientConfigs []string
	var kubeConfig *k8s.KubectlOptions

	for name, ctxConfig := range readinessConfig.Contexts {

		kubeConfig = ctxOptions[name].KubectlOptions
		SetCurrentContext(t, ctxOptions[name].FullName, kubeConfig)

		AddServiceAccount(t, ctxOptions[name], ctxConfig.Namespace, kubeConfig)
		SetupTestArtifactDirectory(t, ctxOptions[name])

		generatedClientConfig := GenerateClientConfig(t, ctxOptions[name])
		generatedClientConfigs = append(generatedClientConfigs, generatedClientConfig)
	}

	CreateConfigs(t, ctxOptions, readinessConfig)

	logger.Log(t, "\n\nK8ssandra: Creating the generic secret ...")
	for name, ctxConfig := range readinessConfig.Contexts {
		kubeConfig = ctxOptions[name].KubectlOptions
		CreateGenericSecret(t, ctxConfig.Namespace, kubeConfig)
	}

	// Apply generated client configs for each cluster.
	for name, ctxConfig := range readinessConfig.Contexts {
		for _, gcc := range generatedClientConfigs {
			SetCurrentContext(t, ctxOptions[name].FullName, ctxOptions[name].KubectlOptions)
			applyClientConfig(t, ctxOptions[name].KubectlOptions, gcc, ctxConfig.Namespace)
		}
	}

	// delete pods, then perform a rollout restart of the operators.
	for name, ctxConfig := range readinessConfig.Contexts {
		SetCurrentContext(t, ctxOptions[name].FullName, ctxOptions[name].KubectlOptions)
		RestartOperator(t, ctxConfig.Namespace, ctxOptions[name].KubectlOptions)
		RestartCassOperator(t, ctxConfig.Namespace, ctxOptions[name].KubectlOptions)
	}
}

func SetupTestArtifactDirectory(t *testing.T, ctxOption model.ContextOption) {

	rootPath := ConfigRootPath(t, ctxOption, "")
	mkdError := os.MkdirAll(rootPath, defaultTempFilePerm)
	require.NoError(t, mkdError, fmt.Sprintf("Unable to setup tmp file location for test artifacts"+
		"root path: %s", rootPath))
}

func CreateConfigs(t *testing.T, ctxOptions map[string]model.ContextOption, readinessConfig model.ReadinessConfig) {

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
		absolutePath := WriteKubeConfig(t, ctxOption, cfg)
		kubeConfig := k8s.NewKubectlOptions(ctxOption.FullName, absolutePath, ctxOption.ServiceAccount.Namespace)

		*(ctxOptions[name].KubectlOptions) = *kubeConfig
		logger.Log(t, fmt.Sprintf("Assigned ctx name: %s to kube config ctx: %s @ %s", name, kubeConfig.ContextName, kubeConfig.ConfigPath))
	}

}

func CreateIdentityEnv(configPath string, identity string, credPath string) map[string]string {
	return map[string]string{
		"KUBECONFIG":                     configPath,
		"GOOGLE_IDENTITY_EMAIL":          identity,
		"GOOGLE_APPLICATION_CREDENTIALS": credPath,
	}
}

func SetCurrentContext(t *testing.T, ctxName string, kubeConfig *k8s.KubectlOptions) bool {
	kubeConfig.Env["KUBECONFIG"] = kubeConfig.ConfigPath
	logger.Log(t, fmt.Sprintf("==== setting current context with kubeconfig target: %s", kubeConfig.Env["KUBECONFIG"]))
	_, err := k8s.RunKubectlAndGetOutputE(t, kubeConfig, "config", "set", "current-context", ctxName)
	require.NoError(t, err, "expecting to set current context without error")
	return err == nil
}

func ConfigRootPath(t *testing.T, contextOption model.ContextOption, fileName string) string {
	rootPath := path.Join(contextOption.ProvisionMeta.ArtifactsRootDir, contextOption.FullName)
	if fileName != "" {
		return path.Join(rootPath, fileName)
	}
	logger.Log(t, fmt.Sprintf("cloud temp context-specific root path: %s", rootPath))
	return rootPath
}

func ConfigCloudTempRootPath(t *testing.T, contextOption model.ContextOption, fileName string) string {
	rootPath := contextOption.ProvisionMeta.ArtifactsRootDir
	if fileName != "" {
		return path.Join(rootPath, fileName)
	}
	logger.Log(t, fmt.Sprintf("cloud temp root path: %s", rootPath))
	return rootPath
}

func CreateGenericSecret(t *testing.T, namespace string, kubeConfig *k8s.KubectlOptions) {
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

func GenerateClientConfig(t *testing.T, ctxOption model.ContextOption) string {
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
	return WriteClientConfig(t, ctxOption, clientConfig)
}

func WriteClientConfig(t *testing.T, ctxOption model.ContextOption, clientConfig model.ClientConfig) string {
	yamlOut, marshalError := yaml.Marshal(&clientConfig)
	if marshalError != nil {
		logger.Log(t, marshalError.Error())
	}

	fileName := ctxOption.ShortName + "-client_config.yaml"
	absoluteFilePath := ConfigRootPath(t, ctxOption, fileName)

	if files.FileExists(absoluteFilePath) {
		err := os.Remove(absoluteFilePath)
		require.NoError(t, err, fmt.Sprintf("Unable to cleanup existing client-config: %s", absoluteFilePath))
	}

	logger.Log(t, fmt.Sprintf("writing client-config to: %s ", absoluteFilePath))
	writeError := ioutil.WriteFile(absoluteFilePath, yamlOut, defaultTempFilePerm)
	require.NoError(t, writeError, fmt.Sprintf("Unable to write client-config: %s", absoluteFilePath))

	return absoluteFilePath
}

func WriteKubeConfig(t *testing.T, ctxOption model.ContextOption, clientConfig v1.Config) string {
	yamlOut, marshalError := yaml.Marshal(&clientConfig)
	if marshalError != nil {
		logger.Log(t, marshalError.Error())
	}

	fileName := defaultKubeConfigFileName
	absoluteFilePath := ConfigCloudTempRootPath(t, ctxOption, fileName)

	if files.FileExists(absoluteFilePath) {
		err := os.Remove(absoluteFilePath)
		require.NoError(t, err, fmt.Sprintf("Unable to cleanup existing kube-config: %s", absoluteFilePath))
	}

	mkdError := os.MkdirAll(ConfigRootPath(t, ctxOption, ""), defaultTempFilePerm)
	require.NoError(t, mkdError, fmt.Sprintf("Unable to setup tmp file location for kube config artifact "+
		"to: %s", absoluteFilePath))
	logger.Log(t, fmt.Sprintf("writing kube config to: %s ", absoluteFilePath))
	writeError := ioutil.WriteFile(absoluteFilePath, yamlOut, defaultTempFilePerm)
	require.NoError(t, writeError, fmt.Sprintf("Unable to write kube config: %s", absoluteFilePath))
	return absoluteFilePath
}

func AddServiceAccount(t *testing.T, ctxOption model.ContextOption, namespace string,
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

func CreateContextOptions(t *testing.T, readinessConfig model.ReadinessConfig,
	provisionMeta model.ProvisionMeta, configs map[string]*k8s.KubectlOptions) map[string]model.ContextOption {

	logger.Log(t, fmt.Sprintf("\n\ncreating all context options for "+
		"provision id: %s", provisionMeta.ProvisionId))

	ctxOptions := map[string]model.ContextOption{}

	for name, ctx := range readinessConfig.Contexts {
		fullName := gcp.ConstructFullContextName(name, ctx.CloudConfig)
		logger.Log(t, fmt.Sprintf("creating context options for context:%s with ns:%s",
			ctx.Name, ctx.Namespace))

		cloudClusterName := gcp.ConstructCloudClusterName(name, ctx.CloudConfig)
		saName := cloudClusterName + "-" + readinessConfig.ServiceAccountNameSuffix + defaultIdentityDomain

		kubeCluster := SelectClusterFromKube(t, name, configs)
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

func SelectClusterFromKube(t *testing.T, name string, configs map[string]*k8s.KubectlOptions) *api.Cluster {

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

func RestartOperator(t *testing.T, namespace string, options *k8s.KubectlOptions) {
	logger.Log(t, "\n\nK8ssandra: restarting k8ssandra-operator")

	pod, err := k8s.RunKubectlAndGetOutputE(t, options, "get", "pod",
		"-l", "app.kubernetes.io/name=k8ssandra-operator", "-n", namespace, "-o", "name")

	if err == nil && pod != "" {
		_, err := k8s.RunKubectlAndGetOutputE(t, options, "delete", pod, "-n", namespace)

		if err != nil {
			logger.Log(t, fmt.Sprintf("WARNING: attempt to delete pod: %s failed due to: %s", pod, err))
		}
	}

	_, err2 := k8s.RunKubectlAndGetOutputE(t, options, "rollout", "restart",
		"deployment", defaultK8ssandraOperatorReleaseName, "-n", namespace)

	require.NoError(t, err2)
	time.Sleep(defaultTimeout)
}

func RestartCassOperator(t *testing.T, namespace string, options *k8s.KubectlOptions) {

	logger.Log(t, "\n\nK8ssandra: restarting k8ssandra-cass-operator")

	// k get pods -n bootz -l app.kubernetes.io/name=cass-operator -o name
	pod, err := k8s.RunKubectlAndGetOutputE(t, options, "get", "pod",
		"-l", "app.kubernetes.io/name=cass-operator", "-n", namespace, "-o", "name")

	if err == nil && pod != "" {
		_, err := k8s.RunKubectlAndGetOutputE(t, options, "delete", pod, "-n", namespace)
		if err != nil {
			logger.Log(t, fmt.Sprintf("WARNING: attempt to delete pod: %s failed due to: %s", pod, err))
		}
	}

	_, err2 := k8s.RunKubectlAndGetOutputE(t, options, "rollout", "restart",
		"deployment", defaultCassandraOperatorName, "-n", namespace)
	require.NoError(t, err2)
	time.Sleep(defaultTimeout)
}

func WaitForEndpoint(t *testing.T, kubeConfig *k8s.KubectlOptions, name string) string {
	out, err := k8s.RunKubectlAndGetOutputE(t, kubeConfig, "get", "ep", name, "-o=jsonpath='{.subsets[0].addresses[0].ip}'")
	require.NoError(t, err, "unexpected error when attempting to obtain endpoint ip availability")
	return out
}

func IsPodRunning(t *testing.T, options *k8s.KubectlOptions, prefixName string) (bool, string) {
	out, err := k8s.RunKubectlAndGetOutputE(t, options, "get", "pod", "--field-selector=status.phase=Running",
		"--no-headers", "-l", "app.kubernetes.io/name=k8ssandra-operator", "-n", options.Namespace,
		"-o", "custom-columns=\":metadata.name\"")

	if err != nil {
		logger.Log(t, fmt.Sprintf("get pod by meta name returned error: %s", err.Error()))
		return false, out
	}

	logger.Log(t, fmt.Sprintf("get running pod by meta name returned: %s", out))
	return out == prefixName, out
}

func applyClientConfig(t *testing.T, options *k8s.KubectlOptions, clientConfigFile string, namespace string) {
	_, err := k8s.RunKubectlAndGetOutputE(t, options, "-n", namespace, "apply", "-f", clientConfigFile)
	require.NoError(t, err)
}
