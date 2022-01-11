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

package util

import (
	"errors"
	"fmt"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/gruntwork-io/terratest/modules/retry"
	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/k8ssandra/cloud-readiness/k8ssandra/test/model"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
	"time"
)

// CheckNodesReady checks for N nodes in ready state with retries having sleep seconds
func CheckNodesReady(t *testing.T, options *k8s.KubectlOptions, expectedNumber int,
	retries int, sleepSecsBetween time.Duration) {
	waitUntilExpectedNodes(t, options, expectedNumber, retries, sleepSecsBetween*time.Second)

	k8s.WaitUntilAllNodesReady(t, options, retries, sleepSecsBetween*time.Second)
	readyNodes := k8s.GetReadyNodes(t, options)
	assert.Equal(t, len(readyNodes), expectedNumber)
}

// CreateOptions constructs Terraform options, which include kubeConfig path.
// Names for cluster, service account, and buckets are made to be specific based on the ID provided.
func CreateOptions(t *testing.T, config model.ReadinessConfig, rootFolder string, kubeConfigPath string) *terraform.Options {
	provConfig := config.ProvisionConfig
	cloudConfig := provConfig.CloudConfig

	uniqueClusterName := strings.ToLower(fmt.Sprintf(config.ClusterNamePrefix+"-%s", config.UniqueId))
	uniqueServiceAccountName := strings.ToLower(fmt.Sprintf(config.ServiceAccountNamePrefix+"-%s", config.UniqueId))
	uniqueBucketName := strings.ToLower(fmt.Sprintf(cloudConfig.Bucket+"-%s", config.UniqueId))

	vars := map[string]interface{}{
		"project_id":              cloudConfig.Project,
		"name":                    cloudConfig.Name,
		"environment":             cloudConfig.Environment,
		"location":                cloudConfig.Location,
		"region":                  cloudConfig.Region,
		"zone":                    cloudConfig.Region,
		"kubectl_config_path":     kubeConfigPath,
		"initial_node_count":      config.ExpectedNodeCount,
		"cluster_name":            uniqueClusterName,
		"service_account":         uniqueServiceAccountName,
		"enable_private_endpoint": false,
		"enable_private_nodes":    false,
		"master_ipv4_cidr_block":  "10.0.0.0/28",
		"machine_type":            cloudConfig.MachineType,
		"bucket_policy_only":      true,
		"role":                    "roles/storage.admin",
		cloudConfig.Bucket:        uniqueBucketName,
	}

	envVars := map[string]string{cloudConfig.CredKey: cloudConfig.CredPath}

	options := terraform.Options{
		TerraformDir: rootFolder,
		Vars:         vars,
		EnvVars:      envVars,
	}
	return &options
}

// checkForAllNodes determines if expected number of nodes exist
func checkForAllNodes(t *testing.T, options *k8s.KubectlOptions, expectedNumber int) (string, error) {
	nodes, err := k8s.GetNodesE(t, options)
	if err != nil {
		return "", err
	}
	if len(nodes) != expectedNumber {
		return "", errors.New("expected nodes NOT found")
	}
	return "all expected nodes are found", nil
}

// waitUntilExpectedNodes polls k8s cluster for an expected number of nodes
func waitUntilExpectedNodes(t *testing.T, options *k8s.KubectlOptions,
	expectedNumber int, retries int, sleepSecsBetween time.Duration) {
	statusMsg := fmt.Sprintf("waiting for %d nodes to be available.", expectedNumber)

	message, err := retry.DoWithRetryE(
		t,
		statusMsg,
		retries,
		sleepSecsBetween,
		func() (string, error) { return checkForAllNodes(t, options, expectedNumber) },
	)
	if err != nil {
		logger.Log(t, "Error waiting for expected number of nodes: %s", err)
		t.Fatal(err)
	}
	logger.Log(t, message)
}
