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

package framework

import (
	"context"
	"fmt"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/k8ssandra/cloud-readiness/k8ssandra/test/model"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/test/kubectl"
	"github.com/k8ssandra/k8ssandra-operator/test/kustomize"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"k8s.io/utils/strings/slices"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"text/template"
	"time"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	replicationapi "github.com/k8ssandra/k8ssandra-operator/apis/replication/v1alpha1"
	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8serrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DefaultControlPlaneContext = "kind-k8ssandra-0"
)

type E2eFramework struct {
	*Framework

	nodeToolStatusUN *regexp.Regexp
}

func NewE2eFramework(t *testing.T, contexts map[string]model.ContextConfig,
	options map[string]*k8s.KubectlOptions) (*E2eFramework, error) {

	controlPlaneContext := ""
	var controlPlaneClient client.Client
	remoteClients := make(map[string]client.Client, 0)

	logger.Log(t, "looking through the range of contexts ... ")
	for name, option := range options {
		config, err := clientcmd.LoadFromFile(option.ConfigPath)
		if err != nil {
			return nil, err
		}

		logger.Log(t, fmt.Sprintf("name: %s", name))
		clientCfg := clientcmd.NewNonInteractiveClientConfig(*config, name, &clientcmd.ConfigOverrides{}, nil)
		restCfg, err := clientCfg.ClientConfig()

		if err != nil {
			return nil, err
		}

		remoteClient, err := client.New(restCfg, client.Options{Scheme: scheme.Scheme})
		if err != nil {
			return nil, err
		}

		if len(controlPlaneContext) == 0 && slices.Contains(contexts[name].ClusterLabels, "control-plane") {
			logger.Log(t, fmt.Sprintf("identified the control-plane cluster: "), name)
			controlPlaneContext = name
			controlPlaneClient = remoteClient
		}
		remoteClients[name] = remoteClient
	}

	require.NotEmpty(t, controlPlaneContext, "Unable to identify the control-plane cluster!")

	f := NewFramework(t, controlPlaneClient, controlPlaneContext, remoteClients)
	re := regexp.MustCompile("UN\\s\\s")
	return &E2eFramework{Framework: f, nodeToolStatusUN: re}, nil
}

// getClusterContexts returns all contexts, including both control plane and data plane.
func (f *E2eFramework) getClusterContexts() []string {
	contexts := make([]string, 0, len(f.remoteClients))
	for ctx, _ := range f.remoteClients {
		contexts = append(contexts, ctx)
	}
	return contexts
}

func (f *E2eFramework) getDataPlaneContexts() []string {
	contexts := make([]string, 0, len(f.remoteClients))
	for ctx, _ := range f.remoteClients {
		if ctx != f.ControlPlaneContext {
			contexts = append(contexts, ctx)
		}
	}
	return contexts
}

type Kustomization struct {
	Namespace string
}

func generateCassOperatorKustomization(namespace string) error {
	tmpl := `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- github.com/k8ssandra/cass-operator/config/default?ref=v1.9.0
namespace: {{ .Namespace }}
`
	k := Kustomization{Namespace: namespace}

	return generateKustomizationFile("cass-operator", k, tmpl)
}

func generateContextsKustomization(namespace string) error {
	tmpl := `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

generatorOptions:
  disableNameSuffixHash: true

secretGenerator:
- files:
  - kubeconfig
  name: k8s-contexts
namespace: {{ .Namespace }}
`
	k := Kustomization{Namespace: namespace}

	if err := generateKustomizationFile("k8s-contexts", k, tmpl); err != nil {
		return err
	}

	src := filepath.Join("..", "..", "build", "in_cluster_kubeconfig")
	dest := filepath.Join("..", "..", "build", "test-config", "k8s-contexts", "kubeconfig")

	buf, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(dest, buf, 0644)
}

func generateK8ssandraOperatorKustomization(namespace string, clusterScoped bool) error {
	controlPlaneDir := ""
	dataPlaneDir := ""
	controlPlaneTmpl := ""
	dataPlaneTmpl := ""

	if clusterScoped {
		controlPlaneDir = "control-plane-cluster-scope"
		dataPlaneDir = "data-plane-cluster-scope"

		controlPlaneTmpl = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- ../../../../config/deployments/control-plane/cluster-scope

components:
- ../../../../config/components/mgmt-api-heap-size
`

		dataPlaneTmpl = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- ../../../../config/deployments/data-plane/cluster-scope

components:
- ../../../../config/components/mgmt-api-heap-size
`
	} else {
		controlPlaneDir = "control-plane"
		dataPlaneDir = "data-plane"

		controlPlaneTmpl = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

namespace: {{ .Namespace }}

resources:
- ../../../../config/deployments/control-plane

components:
- ../../../../config/components/mgmt-api-heap-size
`

		dataPlaneTmpl = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

namespace: {{ .Namespace }}

resources:
- ../../../../config/deployments/data-plane

components:
- ../../../../config/components/mgmt-api-heap-size
`
	}

	k := Kustomization{Namespace: namespace}

	err := generateKustomizationFile(fmt.Sprintf("k8ssandra-operator/%s", controlPlaneDir), k, controlPlaneTmpl)
	if err != nil {
		return err
	}

	return generateKustomizationFile(fmt.Sprintf("k8ssandra-operator/%s", dataPlaneDir), k, dataPlaneTmpl)
}

// generateKustomizationFile Creates the directory <project-root>/build/test-config/<name>
// and generates a kustomization.yaml file using the template tmpl. k defines values that
// will be substituted in the template.
func generateKustomizationFile(name string, k Kustomization, tmpl string) error {
	dir := filepath.Join("..", "..", "build", "test-config", name)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	parsed, err := template.New(name).Parse(tmpl)
	if err != nil {
		return nil
	}

	file, err := os.Create(filepath.Join(dir, "kustomization.yaml"))
	if err != nil {
		return err
	}

	return parsed.Execute(file, k)
}

func (f *E2eFramework) kustomizeAndApply(dir, namespace string, contexts ...string) error {
	kdir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}

	if err := kustomize.SetNamespace(kdir, namespace); err != nil {
		return err
	}

	if len(contexts) == 0 {
		buf, err := kustomize.BuildDir(kdir)
		if err != nil {
			return err
		}

		options := kubectl.Options{Namespace: namespace, Context: DefaultControlPlaneContext, ServerSide: true}
		return kubectl.Apply(options, buf)
	}

	for _, ctx := range contexts {

		buf, err := kustomize.BuildDir(kdir)
		if err != nil {
			return err
		}

		options := kubectl.Options{Namespace: namespace, Context: ctx, ServerSide: true}
		if err := kubectl.Apply(options, buf); err != nil {
			return err
		}
	}

	return nil
}

func (f *E2eFramework) DeployCassandraConfigMap(namespace string) error {
	path := filepath.Join("..", "testdata", "fixtures", "cassandra-config.yaml")

	for _, k8sContext := range f.getClusterContexts() {
		options := kubectl.Options{Namespace: namespace, Context: k8sContext}
		if err := kubectl.Apply(options, path); err != nil {
			return err
		}
	}

	return nil
}

// DeployK8ssandraOperator deploys k8ssandra-operator both in the control plane cluster and
// in the data plane cluster(s). Note that the control plane cluster can also be one of the
// data plane clusters. It then deploys the operator in the data plane clusters with the
// K8ssandraCluster controller disabled. When clusterScoped is true, the operator is
// configured to watch all namespaces and is deployed in the k8ssandra-operator namespace.
func (f *E2eFramework) DeployK8ssandraOperator(namespace string, clusterScoped bool) error {
	if err := generateK8ssandraOperatorKustomization(namespace, clusterScoped); err != nil {
		return err
	}

	baseDir := filepath.Join("..", "..", "build", "test-config", "k8ssandra-operator")
	controlPlane := ""
	dataPlane := ""

	if clusterScoped {
		controlPlane = filepath.Join(baseDir, "control-plane-cluster-scope")
		dataPlane = filepath.Join(baseDir, "data-plane-cluster-scope")
	} else {
		controlPlane = filepath.Join(baseDir, "control-plane")
		dataPlane = filepath.Join(baseDir, "data-plane")
	}

	err := f.kustomizeAndApply(controlPlane, namespace, f.ControlPlaneContext)
	if err != nil {
		return err
	}

	dataPlaneContexts := f.getDataPlaneContexts()
	if len(dataPlaneContexts) > 0 {
		return f.kustomizeAndApply(dataPlane, namespace, dataPlaneContexts...)
	}

	return nil
}

func (f *E2eFramework) DeployCertManager() error {
	dir := filepath.Join("..", "..", "config", "cert-manager", "cert-manager-1.3.1.yaml")

	for _, ctx := range f.getClusterContexts() {
		options := kubectl.Options{Context: ctx}
		if err := kubectl.Apply(options, dir); err != nil {
			return err
		}
	}

	return nil
}

// DeployCassOperator deploys cass-operator in all remote clusters.
func (f *E2eFramework) DeployCassOperator(namespace string) error {
	if err := generateCassOperatorKustomization(namespace); err != nil {
		return err
	}

	dir := filepath.Join("..", "..", "build", "test-config", "cass-operator")

	return f.kustomizeAndApply(dir, namespace, f.getClusterContexts()...)
}

// DeployK8sContextsSecret Deploys the contexts secret in the control plane cluster.
func (f *E2eFramework) DeployK8sContextsSecret(namespace string) error {
	if err := generateContextsKustomization(namespace); err != nil {
		return err
	}

	dir := filepath.Join("..", "..", "build", "test-config", "k8s-contexts")

	return f.kustomizeAndApply(dir, namespace, f.ControlPlaneContext)
}

func (f *E2eFramework) DeployK8sClientConfigs(namespace string) error {
	baseDir, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		return err
	}

	i := 0
	for k8sContext, _ := range f.remoteClients {
		srcCfg := fmt.Sprintf("k8ssandra-%d.yaml", i)
		logger.Log(f.test, fmt.Sprintf("k8sContext: %s", k8sContext))
		cmd := exec.Command(
			filepath.Join("scripts", "create-clientconfig.sh"),
			"--src-kubeconfig", filepath.Join("build", "kubeconfigs", srcCfg),
			"--dest-kubeconfig", filepath.Join("build", "kubeconfigs", "k8ssandra-0.yaml"),
			"--in-cluster-kubeconfig", filepath.Join("build", "kubeconfigs", "updated", srcCfg),
			"--output-dir", filepath.Join("build", "clientconfigs", srcCfg),
			"--namespace", namespace)
		cmd.Dir = baseDir

		fmt.Println(cmd)

		output, err := cmd.CombinedOutput()
		fmt.Println(string(output))

		if err != nil {
			return err
		}

		i++
	}

	return nil
}

// DeleteNamespace Deletes the namespace from all remote clusters and blocks until they
// have completely terminated.
func (f *E2eFramework) DeleteNamespace(name string, timeout, interval time.Duration) error {
	// TODO Make sure we delete from the control plane cluster as well

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	for _, remoteClient := range f.remoteClients {
		if err := remoteClient.Delete(context.Background(), namespace.DeepCopy()); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	// Should this wait.Poll call be per cluster?
	return wait.Poll(interval, timeout, func() (bool, error) {
		for _, remoteClient := range f.remoteClients {
			err := remoteClient.Get(context.TODO(), types.NamespacedName{Name: name}, namespace.DeepCopy())

			if err == nil || !apierrors.IsNotFound(err) {
				return false, nil
			}
		}

		return true, nil
	})
}

func (f *E2eFramework) WaitForCrdsToBecomeActive() error {
	// TODO Add multi-cluster support.
	// By default this should wait for all clusters including the control plane cluster.

	return kubectl.WaitForCondition("established", "--timeout=60s", "--all", "crd")
}

// WaitForK8ssandraOperatorToBeReady blocks until the k8ssandra-operator deployment is
// ready in the control plane cluster.
func (f *E2eFramework) WaitForK8ssandraOperatorToBeReady(namespace string, timeout, interval time.Duration) error {
	key := ClusterKey{
		K8sContext:     f.ControlPlaneContext,
		NamespacedName: types.NamespacedName{Namespace: namespace, Name: "k8ssandra-operator"},
	}
	return f.WaitForDeploymentToBeReady(f.test, key, timeout, interval)
}

func (f *E2eFramework) WaitForCertManagerToBeReady(namespace string, timeout, interval time.Duration) error {
	key := ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "cert-manager"}}
	if err := f.WaitForDeploymentToBeReady(f.test, key, timeout, interval); err != nil {
		return nil
	}

	key.NamespacedName.Name = "cert-manager-cainjector"
	if err := f.WaitForDeploymentToBeReady(f.test, key, timeout, interval); err != nil {
		return err
	}

	key.NamespacedName.Name = "cert-manager-webhook"
	if err := f.WaitForDeploymentToBeReady(f.test, key, timeout, interval); err != nil {
		return err
	}

	return nil
}

// WaitForCassOperatorToBeReady blocks until the cass-operator deployment is ready in all
// clusters.
func (f *E2eFramework) WaitForCassOperatorToBeReady(namespace string, timeout, interval time.Duration) error {
	key := ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "cass-operator-controller-manager"}}
	return f.WaitForDeploymentToBeReady(f.test, key, timeout, interval)
}

// DumpClusterInfo Executes `kubectl cluster-info dump -o yaml` on each cluster. The output
// is stored under <project-root>/build/test.
func (f *E2eFramework) DumpClusterInfo(test string, namespace ...string) error {

	now := time.Now()
	testDir := strings.ReplaceAll(test, "/", "_")
	baseDir := fmt.Sprintf("../../build/test/%s/%d-%d-%d-%d-%d", testDir, now.Year(), now.Month(), now.Day(), now.Hour(), now.Second())
	errs := make([]error, 0)

	for ctx, _ := range f.remoteClients {
		outputDir := filepath.Join(baseDir, ctx)
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			errs = append(errs, fmt.Errorf("failed to make output for cluster %s: %w", ctx, err))
			return err
		}

		opts := kubectl.ClusterInfoOptions{Options: kubectl.Options{Context: ctx}, OutputDirectory: outputDir}
		if len(namespace) == 1 {
			opts.Namespace = namespace[0]
		} else {
			opts.Namespaces = namespace
		}

		if err := kubectl.DumpClusterInfo(opts); err != nil {
			errs = append(errs, fmt.Errorf("failed to dump cluster info for cluster %s: %w", ctx, err))
		}
	}

	if len(errs) > 0 {
		return k8serrors.NewAggregate(errs)
	}
	return nil
}

// DeleteDatacenters deletes all CassandraDatacenters in namespace in all remote clusters.
// This function blocks until all pods from all CassandraDatacenters have terminated.
func (f *E2eFramework) DeleteDatacenters(namespace string, timeout, interval time.Duration) error {
	return f.deleteAllResources(
		namespace,
		&cassdcapi.CassandraDatacenter{},
		timeout,
		interval,
		client.HasLabels{cassdcapi.ClusterLabel},
	)
}

// DeleteStargates deletes all Stargates in namespace in all remote clusters.
// This function blocks until all pods from all Stargates have terminated.
func (f *E2eFramework) DeleteStargates(namespace string, timeout, interval time.Duration) error {
	return f.deleteAllResources(
		namespace,
		&stargateapi.Stargate{},
		timeout,
		interval,
		client.HasLabels{stargateapi.StargateLabel},
	)
}

// DeleteReapers deletes all Reapers in namespace in all remote clusters.
// This function blocks until all pods from all Reapers have terminated.
func (f *E2eFramework) DeleteReapers(namespace string, timeout, interval time.Duration) error {
	return f.deleteAllResources(
		namespace,
		&reaperapi.Reaper{},
		timeout,
		interval,
		client.HasLabels{reaperapi.ReaperLabel},
	)
}

// DeleteReplicatedSecrets deletes all the ReplicatedSecrets in the namespace. This causes
// some delay while secret controller removes the finalizers and clears the replicated secrets from
// remote clusters.
func (f *E2eFramework) DeleteReplicatedSecrets(namespace string, timeout, interval time.Duration) error {
	if err := f.Client.DeleteAllOf(context.Background(), &replicationapi.ReplicatedSecret{}, client.InNamespace(namespace)); err != nil {
		return err
	}

	return wait.Poll(interval, timeout, func() (bool, error) {
		list := &replicationapi.ReplicatedSecretList{}
		if err := f.Client.List(context.Background(), list, client.InNamespace(namespace)); err != nil {
			return false, err
		}
		return len(list.Items) == 0, nil
	})
}

func (f *E2eFramework) DeleteK8ssandraOperatorPods(namespace string, timeout, interval time.Duration) error {
	if err := f.Client.DeleteAllOf(context.TODO(), &corev1.Pod{}, client.InNamespace(namespace), client.MatchingLabels{"control-plane": "k8ssandra-operator"}); err != nil {
		return err
	}
	return nil
}

func (f *E2eFramework) deleteAllResources(
	namespace string,
	resource client.Object,
	timeout, interval time.Duration,
	podListOptions ...client.ListOption,
) error {
	for _, remoteClient := range f.remoteClients {
		if err := remoteClient.DeleteAllOf(context.TODO(), resource, client.InNamespace(namespace)); err != nil {
			// If the CRD wasn't deployed at all to this cluster, keep going
			if _, ok := err.(*meta.NoKindMatchError); !ok {
				return err
			}
		}
	}
	// Check that all pods created by resources are terminated
	// FIXME Should there be a separate wait.Poll call per cluster?
	return wait.Poll(interval, timeout, func() (bool, error) {
		podListOptions = append(podListOptions, client.InNamespace(namespace))
		for k8sContext, remoteClient := range f.remoteClients {
			list := &corev1.PodList{}
			if err := remoteClient.List(context.TODO(), list, podListOptions...); err != nil {
				logger.Log(f.test, "failed to list pods for context: %s with error: %s  ", k8sContext, err.Error())
				return false, nil
			}
			if len(list.Items) > 0 {
				return false, nil
			}
		}
		return true, nil
	})
}

func (f *E2eFramework) UndeployK8ssandraOperator(namespace string) error {
	dir, err := filepath.Abs("../testdata/k8ssandra-operator")
	if err != nil {
		return err
	}

	buf, err := kustomize.BuildDir(dir)
	if err != nil {
		return err
	}

	options := kubectl.Options{Namespace: namespace}

	return kubectl.Delete(options, buf)
}

// GetNodeToolStatusUN Executes nodetool status against the Cassandra pod and returns a
// count of the matching lines reporting a status of Up/Normal.
func (f *E2eFramework) GetNodeToolStatusUN(opts kubectl.Options, pod string) (int, error) {
	output, err := kubectl.Exec(opts, pod, "nodetool", "status")
	if err != nil {
		return -1, err
	}

	matches := f.nodeToolStatusUN.FindAllString(output, -1)

	return len(matches), nil
}

// WaitForNodeToolStatusUN polls until nodetool status reports UN for count nodes.
func (f *E2eFramework) WaitForNodeToolStatusUN(opts kubectl.Options, pod string, count int, timeout, interval time.Duration) error {
	return wait.Poll(interval, timeout, func() (bool, error) {
		actual, err := f.GetNodeToolStatusUN(opts, pod)
		if err != nil {
			return false, err
		}
		return actual == count, nil
	})

}