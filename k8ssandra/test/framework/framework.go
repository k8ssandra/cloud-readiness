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
	"github.com/gruntwork-io/terratest/modules/logger"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	configapi "github.com/k8ssandra/k8ssandra-operator/apis/config/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	replicationapi "github.com/k8ssandra/k8ssandra-operator/apis/replication/v1alpha1"
	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
	"time"
)

var (
	Client client.Client
)

// TODO Add a Framework type and make functions method on that type
// By making these functions methods we can pass the testing.T and namespace arguments just
// once in the constructor. We can also include defaults for the timeout and interval
// parameters that show up in multiple functions.

func Init(t *testing.T) {
	var err error

	err = api.AddToScheme(scheme.Scheme)
	require.NoError(t, err, "failed to register scheme for k8ssandra-operator")

	err = stargateapi.AddToScheme(scheme.Scheme)
	require.NoError(t, err, "failed to register scheme for stargate")

	err = reaperapi.AddToScheme(scheme.Scheme)
	require.NoError(t, err, "failed to register scheme for reaper")

	err = configapi.AddToScheme(scheme.Scheme)
	require.NoError(t, err, "failed to register scheme for k8ssandra-operator configs")

	err = replicationapi.AddToScheme(scheme.Scheme)
	require.NoError(t, err, "failed to register scheme for k8ssandra-operator replication")

	err = cassdcapi.AddToScheme(scheme.Scheme)
	require.NoError(t, err, "failed to register scheme for cass-operator")

	// cfg, err := ctrl.GetConfig()
	// require.NoError(t, err, "failed to get *rest.Config")
	//
	// Client, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	// require.NoError(t, err, "failed to create controller-runtime client")
}

// Framework provides methods for use in both integration and e2e tests.
type Framework struct {
	// Client is the client for the control plane cluster, i.e., the cluster in which the
	// K8ssandraCluster controller is deployed. Note that this may also be one of the
	// remote clusters.
	Client client.Client

	// The Kubernetes context in which the K8ssandraCluser controller is running.
	ControlPlaneContext string

	ControlPlanNamespace string

	// RemoteClients is mapping of Kubernetes context names to clients.
	remoteClients map[string]client.Client

	test *testing.T
}

type ClusterKey struct {
	types.NamespacedName

	K8sContext string
}

func (k ClusterKey) String() string {
	return k.K8sContext + string(types.Separator) + k.Namespace + string(types.Separator) + k.Name
}

func NewFramework(t *testing.T, client client.Client, controlPlanContext string,
	remoteClients map[string]client.Client) *Framework {
	return &Framework{Client: client, ControlPlaneContext: controlPlanContext, remoteClients: remoteClients, test: t}
}

// Get fetches the object specified by key from the cluster specified by key. An error is
// returned is ClusterKey.K8sContext is not set or if there is no corresponding client.
func (f *Framework) Get(ctx context.Context, key ClusterKey, obj client.Object) error {
	if len(key.K8sContext) == 0 {
		return fmt.Errorf("the K8sContext must be specified for key %s", key)
	}

	remoteClient, found := f.remoteClients[key.K8sContext]
	if !found {
		return fmt.Errorf("no remote client found for context %s", key.K8sContext)
	}

	return remoteClient.Get(ctx, key.NamespacedName, obj)
}

func (f *Framework) CreateNamespace(name string) error {
	for k8sContext, remoteClient := range f.remoteClients {
		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}
		f.test.Log(f.test, "creating namespace", "Namespace", name, "Context", k8sContext)
		if err := remoteClient.Create(context.Background(), namespace); err != nil {
			return err
		}
	}
	return nil
}

func (f *Framework) k8sContextNotFound(k8sContext string) error {
	return fmt.Errorf("context %s not found", k8sContext)
}

// SetDatacenterStatusReady fetches the CassandraDatacenter specified by key and persists
// a status update to make the CassandraDatacenter ready.
func (f *Framework) SetDatacenterStatusReady(ctx context.Context, key ClusterKey) error {
	now := metav1.Now()
	return f.PatchDatacenterStatus(ctx, key, func(dc *cassdcapi.CassandraDatacenter) {
		dc.Status.CassandraOperatorProgress = cassdcapi.ProgressReady
		dc.SetCondition(cassdcapi.DatacenterCondition{
			Type:               cassdcapi.DatacenterReady,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: now,
		})
	})
}

// PatchDatacenterStatus fetches the datacenter specified by key, applies changes via
// updateFn, and then performs a patch operation. key.K8sContext must be set and must
// have a corresponding client.
func (f *Framework) PatchDatacenterStatus(ctx context.Context, key ClusterKey, updateFn func(dc *cassdcapi.CassandraDatacenter)) error {
	dc := &cassdcapi.CassandraDatacenter{}
	err := f.Get(ctx, key, dc)

	if err != nil {
		return err
	}

	patch := client.MergeFromWithOptions(dc.DeepCopy(), client.MergeFromWithOptimisticLock{})
	updateFn(dc)

	remoteClient := f.remoteClients[key.K8sContext]
	return remoteClient.Status().Patch(ctx, dc, patch)
}

func (f *Framework) PatchStargateStatus(ctx context.Context, key ClusterKey, updateFn func(sg *stargateapi.Stargate)) error {
	sg := &stargateapi.Stargate{}
	err := f.Get(ctx, key, sg)

	if err != nil {
		return err
	}

	patch := client.MergeFromWithOptions(sg.DeepCopy(), client.MergeFromWithOptimisticLock{})
	updateFn(sg)

	remoteClient := f.remoteClients[key.K8sContext]
	return remoteClient.Status().Patch(ctx, sg, patch)
}

func (f *Framework) PatchReaperStatus(ctx context.Context, key ClusterKey, updateFn func(sg *reaperapi.Reaper)) error {
	sg := &reaperapi.Reaper{}
	err := f.Get(ctx, key, sg)

	if err != nil {
		return err
	}

	patch := client.MergeFromWithOptions(sg.DeepCopy(), client.MergeFromWithOptimisticLock{})
	updateFn(sg)

	remoteClient := f.remoteClients[key.K8sContext]
	return remoteClient.Status().Patch(ctx, sg, patch)
}

// WaitForDeploymentToBeReady Blocks until the Deployment is ready. If
// ClusterKey.K8sContext is empty, this method blocks until the deployment is ready in all
// remote clusters.
func (f *Framework) WaitForDeploymentToBeReady(t *testing.T, key ClusterKey, timeout, interval time.Duration) error {
	return wait.Poll(interval, timeout, func() (bool, error) {
		if len(key.K8sContext) == 0 {
			for _, remoteClient := range f.remoteClients {
				deployment := &appsv1.Deployment{}
				if err := remoteClient.Get(context.TODO(), key.NamespacedName, deployment); err != nil {
					f.test.Error("failed to get deployment", "key", key)
					return false, err
				}

				if deployment.Status.Replicas != deployment.Status.ReadyReplicas {
					return false, nil
				}
			}

			return true, nil
		}

		remoteClient, found := f.remoteClients[key.K8sContext]
		if !found {
			return false, f.k8sContextNotFound(key.K8sContext)
		}

		deployment := &appsv1.Deployment{}
		if err := remoteClient.Get(context.TODO(), key.NamespacedName, deployment); err != nil {
			f.test.Error(err, "failed to get deployment", "key", key)
			return false, err
		}
		return deployment.Status.Replicas == deployment.Status.ReadyReplicas, nil
	})
}

func (f *Framework) DeleteK8ssandraCluster(ctx context.Context, key client.ObjectKey) error {
	kc := &api.K8ssandraCluster{}
	err := f.Client.Get(ctx, key, kc)
	if err != nil {
		return err
	}
	return f.Client.Delete(ctx, kc)
}

func (f *Framework) DeleteK8ssandraClusters(t *testing.T, namespace string, interval, timeout time.Duration) error {
	f.test.Log("Deleting K8ssandraClusters", "Namespace", namespace)
	k8ssandra := &api.K8ssandraCluster{}

	if err := f.Client.DeleteAllOf(context.TODO(), k8ssandra, client.InNamespace(namespace)); err != nil {
		logger.Log(f.test, fmt.Sprintf("Failed to delete K8ssandraClusters due to error: %s", err.Error()))
		return err
	}

	return wait.Poll(interval, timeout, func() (bool, error) {
		list := &api.K8ssandraClusterList{}
		err := f.Client.List(context.Background(), list, client.InNamespace(namespace))
		if err != nil {
			logger.Log(f.test, fmt.Sprintf("Waiting for k8ssandracluster deletion error: %s", err.Error()))
			return false, nil
		}
		return len(list.Items) == 0, nil
	})
}

func (f *Framework) DeleteCassandraDatacenters(namespace string, interval, timeout time.Duration) error {
	logger.Log(f.test, fmt.Sprintf("Deleting CassandraDatacenters for ns: %s"), namespace)
	dc := &cassdcapi.CassandraDatacenter{}

	if err := f.Client.DeleteAllOf(context.Background(), dc, client.InNamespace(namespace)); err != nil {
		logger.Log(f.test, fmt.Sprintf("Failed to delete CassandraDatacenters for ns: %s", namespace))
	}

	return wait.Poll(interval, timeout, func() (bool, error) {
		list := &cassdcapi.CassandraDatacenterList{}
		err := f.Client.List(context.Background(), list, client.InNamespace(namespace))
		if err != nil {
			logger.Log(f.test, fmt.Sprintf("Waiting for CassandraDatacenter deletion.  Error: %s", err.Error()))
			return false, nil
		}
		return len(list.Items) == 0, nil
	})
}

// NewWithDatacenter is a function generator for withDatacenter that is bound to ctx, and key.
func (f *Framework) NewWithDatacenter(ctx context.Context, key ClusterKey) func(func(*cassdcapi.CassandraDatacenter) bool) func() bool {
	return func(condition func(dc *cassdcapi.CassandraDatacenter) bool) func() bool {
		return f.withDatacenter(ctx, key, condition)
	}
}

// withDatacenter Fetches the CassandraDatacenter specified by key and then calls condition.
func (f *Framework) withDatacenter(ctx context.Context, key ClusterKey, condition func(*cassdcapi.CassandraDatacenter) bool) func() bool {
	return func() bool {
		remoteClient, found := f.remoteClients[key.K8sContext]
		if !found {
			logger.Log(f.test, fmt.Sprintf("ctx not found: %s, cannot lookup CassandraDatacenter  key: %s",
				f.k8sContextNotFound(key.K8sContext).Error(), key))
			return false
		}

		dc := &cassdcapi.CassandraDatacenter{}
		if err := remoteClient.Get(ctx, key.NamespacedName, dc); err == nil {
			return condition(dc)
		} else {
			if !errors.IsNotFound(err) {
				// We won't log the error if its not found because that is expected and it helps cut
				// down on the verbosity of the test output.
				logger.Log(f.test, fmt.Sprintf("failed to get CassandraDatacenter key: %s", key))
			}
			return false
		}
	}
}

func (f *Framework) DatacenterExists(ctx context.Context, key ClusterKey) func() bool {
	withDc := f.NewWithDatacenter(ctx, key)
	return withDc(func(dc *cassdcapi.CassandraDatacenter) bool {
		return true
	})
}

// NewWithStargate is a function generator for withStargate that is bound to ctx, and key.
func (f *Framework) NewWithStargate(ctx context.Context, key ClusterKey) func(func(stargate *stargateapi.Stargate) bool) func() bool {
	return func(condition func(*stargateapi.Stargate) bool) func() bool {
		return f.withStargate(ctx, key, condition)
	}
}

// withStargate Fetches the stargate specified by key and then calls condition.
func (f *Framework) withStargate(ctx context.Context, key ClusterKey, condition func(*stargateapi.Stargate) bool) func() bool {
	return func() bool {
		remoteClient, found := f.remoteClients[key.K8sContext]
		if !found {
			logger.Log(f.test, fmt.Sprintf("issue: %s cannot lookup Stargate key: %s", f.k8sContextNotFound(key.K8sContext).Error(), key))
			return false
		}
		stargate := &stargateapi.Stargate{}
		if err := remoteClient.Get(ctx, key.NamespacedName, stargate); err == nil {
			return condition(stargate)
		} else {
			logger.Log(f.test, fmt.Sprintf("failed to get Stargate key: %s", key))
			return false
		}
	}
}

func (f *Framework) StargateExists(ctx context.Context, key ClusterKey) func() bool {
	withStargate := f.NewWithStargate(ctx, key)
	return withStargate(func(s *stargateapi.Stargate) bool {
		return true
	})
}

// NewWithReaper is a function generator for withReaper that is bound to ctx, and key.
func (f *Framework) NewWithReaper(ctx context.Context, key ClusterKey) func(func(reaper *reaperapi.Reaper) bool) func() bool {
	return func(condition func(*reaperapi.Reaper) bool) func() bool {
		return f.withReaper(ctx, key, condition)
	}
}

// withReaper Fetches the reaper specified by key and then calls condition.
func (f *Framework) withReaper(ctx context.Context, key ClusterKey, condition func(*reaperapi.Reaper) bool) func() bool {
	return func() bool {
		remoteClient, found := f.remoteClients[key.K8sContext]
		if !found {
			logger.Log(f.test, fmt.Sprintf("issue: %s cannot lookup Reaper key: %s", f.k8sContextNotFound(key.K8sContext).Error(), key))
			return false
		}
		reaper := &reaperapi.Reaper{}
		if err := remoteClient.Get(ctx, key.NamespacedName, reaper); err == nil {
			return condition(reaper)
		} else {
			return false
		}
	}
}

func (f *Framework) ReaperExists(ctx context.Context, key ClusterKey) func() bool {
	withReaper := f.NewWithReaper(ctx, key)
	return withReaper(func(r *reaperapi.Reaper) bool {
		return true
	})
}


