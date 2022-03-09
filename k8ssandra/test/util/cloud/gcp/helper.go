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

package gcp

import (
	"fmt"
	_ "github.com/gruntwork-io/terratest/modules/gcp"
	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/gruntwork-io/terratest/modules/shell"
	"github.com/k8ssandra/cloud-readiness/k8ssandra/test/model"
	"testing"
)

func ConstructFullContextName(contextName string, config model.ReadinessConfig) string {
	return "gke_" + config.ProvisionConfig.CloudConfig.Project + "_" + config.ProvisionConfig.CloudConfig.Region + "_" +
		config.ProvisionConfig.CloudConfig.Environment + "-" + contextName
}

func ConstructCloudClusterName(contextName string, config model.CloudConfig) string {
	return config.Environment + "-" + contextName
}

func FetchCreds(t *testing.T, readinessConfig model.ReadinessConfig, env map[string]string, clusterName string) bool {
	region := readinessConfig.ProvisionConfig.CloudConfig.Region
	project := readinessConfig.ProvisionConfig.CloudConfig.Project
	args := []string{"container", "clusters", "get-credentials", clusterName, "--region", region, "--project", project}
	var cmd = shell.Command{
		Command:    "gcloud",
		Args:       args,
		WorkingDir: "/tmp",
		Env:        env,
		Logger:     logger.Default,
	}
	_, cmdErr := shell.RunCommandAndGetOutputE(t, cmd)
	if cmdErr != nil {
		logger.Log(t, fmt.Sprintf("failed service account activation key create: %s", cmdErr))
		return false
	}
	return true

}

func Switch(t *testing.T, serviceAccount string, env map[string]string) bool {
	args := []string{"config", "set", "account", serviceAccount}
	var cmd = shell.Command{
		Command:    "gcloud",
		Args:       args,
		WorkingDir: "/tmp",
		Env:        env,
		Logger:     logger.Default,
	}
	_, cmdErr := shell.RunCommandAndGetOutputE(t, cmd)
	if cmdErr != nil {
		logger.Log(t, fmt.Sprintf("failed service account switch to: %s", serviceAccount))
		return false
	}
	logger.Log(t, fmt.Sprintf("switched to service account: %s", serviceAccount))
	return true
}
