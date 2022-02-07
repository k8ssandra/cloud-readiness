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
	"github.com/gruntwork-io/terratest/modules/gcp"
	_ "github.com/gruntwork-io/terratest/modules/gcp"
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

func FetchCreds(t *testing.T) string {
	return gcp.GetGoogleCredentialsFromEnvVar(t)
}


