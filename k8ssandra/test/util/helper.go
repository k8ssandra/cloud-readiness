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
	"errors"
	"fmt"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/gruntwork-io/terratest/modules/retry"
	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/k8ssandra/cloud-readiness/k8ssandra/test/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log"
	"strings"
	"testing"
	"time"
)

// CheckNodesReady checks for N nodes in ready state with retries having sleep seconds
func CheckNodesReady(t *testing.T, options *k8s.KubectlOptions, expectedNumber int,
	retries int, sleepSecsBetween int) {
	waitUntilExpectedNodes(t, options, expectedNumber, retries, time.Duration(sleepSecsBetween)*time.Second)

	k8s.WaitUntilAllNodesReady(t, options, retries, time.Duration(sleepSecsBetween)*time.Second)
	readyNodes := k8s.GetReadyNodes(t, options)
	assert.Equal(t, len(readyNodes), expectedNumber)
}

// CreateOptions constructs Terraform options, which include kubeConfig path.
// Names for cluster, service account, and buckets are made to be specific based on the ID provided.
func CreateOptions(config model.ReadinessConfig, rootFolder string,
	kubeConfigPath string) map[string]*terraform.Options {

	provConfig := config.ProvisionConfig
	cloudConfig := provConfig.CloudConfig

	var tfOptions = map[string]*terraform.Options{}
	for name := range config.Contexts {

		uniqueClusterName := strings.ToLower(fmt.Sprintf(name))
		uniqueServiceAccountName := strings.ToLower(fmt.Sprintf(config.ServiceAccountNamePrefix+"-%s", config.UniqueId))
		uniqueBucketName := strings.ToLower(fmt.Sprintf(cloudConfig.Bucket+"-%s", config.UniqueId))

		vars := map[string]interface{}{
			"project_id":              cloudConfig.Project,
			"name":                    uniqueClusterName,
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
		tfOptions[name] = &options
	}
	return tfOptions
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

func fetchCertificate(t *testing.T, options *k8s.KubectlOptions, secret string) string {

	logger.Log(t, fmt.Sprintf("obtaining certificate with secret: %s", secret))
	out, err := k8s.RunKubectlAndGetOutputE(t, options, "get", "secret", secret, "-o", "jsonpath={.data['ca\\.crt']}")

	require.NoError(t, err)
	require.NotNil(t, out)
	require.NotEmpty(t, out)

	logger.Log(t, fmt.Sprintf("certificate obtained: %s", out))
	return out
}

func fetchToken(t *testing.T, options *k8s.KubectlOptions, secret string) string {

	out, err := k8s.RunKubectlAndGetOutputE(t, options, "--context", options.ContextName,
		"-n", options.Namespace, "get", "secret", secret, "-o", "jsonpath={.data.token}")

	require.NoError(t, err)
	require.NotNil(t, out)

	decoded, err := base64.StdEncoding.DecodeString(out)
	if err != nil {
		log.Fatalf("Some error occured during base64 decode. Error %s", err.Error())
	}
	return string(decoded)
}

func fetchSecret(t *testing.T, options *k8s.KubectlOptions, serviceAccount string) string {

	out, err := k8s.RunKubectlAndGetOutputE(t, options,
		"get", "serviceaccount", serviceAccount, "-o", "jsonpath={.secrets[0].name}")

	require.NoError(t, err)
	require.NotNil(t, out)
	return out
}
