package util

import (
	"fmt"
	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/k8ssandra/cloud-readiness/k8ssandra/test/framework"
	"github.com/k8ssandra/cloud-readiness/k8ssandra/test/model"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/strings/slices"
	"strconv"
	"testing"
)

func InstallK8ssandra(t *testing.T, config model.ReadinessConfig, serviceAccount string) {

	// 1) Install Cert Manager
	// kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.5.3/cert-manager.yaml

	// 2) Install the operator control-plane
	// Target the control-plane active context, then apply
	// kubectl apply -k github.com/k8ssandra/config/deployments/control-plane

	// At this point, the cass-operator and k8ssandra-operator are deployed.

	// 3) Install the data-plane
	// Loop over data-plane contexts we have built.
	// Install the operator on each.
	// kubectl apply -k github.com/k8ssandra/config/deployments/data-plane
	// This installs the operator in the k8ssandra-operator namespace.

	require.NotEmpty(t, serviceAccount, "installation of k8ssandra requires "+
		"that service account is existing.")

	logger.Log(t, "install started for k8ssandra")
	// provConfig := config.ProvisionConfig
	// k8cConfig := provConfig.K8cConfig
	options := createKubeConfigs(t, config)

	logger.Log(t, "e2e framework integration from k8ssandra-operator")
	e2eFramework, err := framework.NewE2eFramework(t, config.Contexts, options)

	require.NoError(t, err)
	require.NotNil(t, e2eFramework)

	var isRepoSetup = false
	for name, ctx := range config.Contexts {

		kubeOptions := options[name].KubeOptions
		isControlPlane := slices.Contains(ctx.ClusterLabels, "control-plane")
		helmOptions := createHelmOptions(kubeOptions, map[string]string{"controlPlane": strconv.FormatBool(isControlPlane)})

		if !isRepoSetup {
			isRepoSetup = repoSetup(t, helmOptions)
		}

		// jb -verified
		// installCertManager(t, kubeOptions)

		kubeOptions = options[name].KubeOptions
		// TODO - REFERENCE https://github.com/k8ssandra/k8ssandra-operator/blob/main/docs/install/README.md#multi-cluster-2
		if isControlPlane {
			installControlPlane(t, kubeOptions)
			// deployK8ssandraCluster(t, config, name, kubeOptions)
		} else {
			installDataPlane(t, kubeOptions)
			//installK8ssandraOperator(t, name, ctx.Namespace, helmOptions,
			//	k8cConfig.ClusterScoped)
		}
	}
	controlPlaneContext := e2eFramework.ControlPlaneContext
	logger.Log(t, fmt.Sprintf("control plane identified as: %s", controlPlaneContext))

	// verifyControlPlane(t, controlPlaneContext)
	// fetchServiceAccountConfig(t, options, controlPlaneContext, serviceAccount)
}

func installDataPlane(t *testing.T, options *k8s.KubectlOptions) {
	out, err := k8s.RunKubectlAndGetOutputE(t, options, "-n", options.Namespace,
		"apply", "-k", defaultK8ssandraDataPlane)
	require.NoError(t, err)
	require.NotNil(t, out)
}

func installCertManager(t *testing.T, options *k8s.KubectlOptions) {
	options.Namespace = ""
	_, err := k8s.RunKubectlAndGetOutputE(t, options,
		"apply", "-f", defaultCertManagerFile)
	require.NoError(t, err)
}

func installControlPlane(t *testing.T, options *k8s.KubectlOptions) {
	out, err := k8s.RunKubectlAndGetOutputE(t, options, "-n", options.Namespace,
		"apply", "-k", defaultK8ssandraControlPlane)
	require.NoError(t, err)
	require.NotNil(t, out)
}

/**
Create the k8ssandra-operator namespace if necessary
Install cass-operator in the k8ssandra-operator namespace
Install K8ssandra-Operator in the k8ssandra-operator namespace OR in the context namespace depending on scope
The cluster scoping is required for clusters that run locally (e.g. kind, etc.)
*/
func installK8ssandraOperator(t *testing.T, contextName string, namespace string,
	helmOptions *helm.Options, isClusterScoped bool) {

	logger.Log(t, fmt.Sprintf("installing k8ssandra-operator "+
		"for context: [%s] namespace: [%s]", contextName, namespace))
	logger.Log(t, fmt.Sprintf("installing [k8ssandra-operator] "+
		"for context: [%s] and namespace: [%s]", contextName, namespace))

	var ns = namespace
	logger.Log(t, fmt.Sprintf("cluster scoped for k8ssandra-operator is set as: %s",
		strconv.FormatBool(isClusterScoped)))
	//if isClusterScoped {
	//	ns = defaultK8ssandraOperatorReleaseName
	//}

	result, err := helm.RunHelmCommandAndGetOutputE(t, helmOptions, "install",
		defaultK8ssandraOperatorReleaseName, defaultK8ssandraOperatorChart, "-n", ns,
		"--create-namespace")

	if err != nil {
		logger.Log(t, fmt.Sprintf("failed k8ssandra install due to error: %s", err.Error()))
	} else {
		logger.Log(t, fmt.Sprintf("installation result: %s", result))
	}
}
