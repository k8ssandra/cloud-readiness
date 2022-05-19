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
	"fmt"
	"github.com/gruntwork-io/terratest/modules/files"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/logger"
	ts "github.com/gruntwork-io/terratest/modules/test-structure"
	"github.com/k8ssandra/cloud-readiness/k8ssandra/test/model"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strconv"
	"testing"
)

func DeleteResource(t *testing.T, kubeConfig *k8s.KubectlOptions, resourceKind string, resourceName string) {

	require.NotEmpty(t, resourceKind, "required resource kind to be specified for delete")
	require.NotEmpty(t, resourceName, "required resource name to be specified for delete")

	_, err := k8s.RunKubectlAndGetOutputE(t, kubeConfig, "delete", resourceKind, resourceName)
	if err != nil {
		logger.Log(t, fmt.Sprintf("WARNING: attempt to delete resource of kind: %s "+
			"and name: %s failed: %s", resourceKind, resourceName, err.Error()))
	}
}

func RemoveProvisioningArtifacts(t *testing.T, meta model.ProvisionMeta, readinessConfig model.ReadinessConfig,
	isCloudCleanRequested bool) {

	logger.Log(t, fmt.Sprintf("remove provisioning artifacts with cloud clean request: %s",
		strconv.FormatBool(isCloudCleanRequested)))

	// Remove tmp artifacts first, followed by overall test manifest unless issue detected
	if removeTempArtifacts(t, meta, readinessConfig, isCloudCleanRequested) {
		removeManifestFolder(t, meta)
	}
}

func removeTempArtifacts(t *testing.T, meta model.ProvisionMeta, readinessConfig model.ReadinessConfig,
	isCloudCleanRequested bool) bool {

	var isSuccess = true
	artifacts, err := ioutil.ReadDir(path.Join(meta.ArtifactsRootDir, ".test-data"))

	if err != nil {
		logger.Log(t, fmt.Sprintf("WARNING: Unable to locate the '.test-data' in this test artifact.  "+
			"Not removing to ensure verficiation before delete in: %s.  A manual removal of the test "+
			"artifacts is required.", meta.ArtifactsRootDir))
		return false
	}

	for _, artifact := range artifacts {

		artifactPath := ts.FormatTestDataPath(meta.ArtifactsRootDir, artifact.Name())
		logger.Log(t, fmt.Sprintf("removing tmp artifacts in dir: %s for artifact: %s looking for: %s",
			meta.ArtifactsRootDir, artifact.Name(), artifactPath))

		if artifactPath != "" && files.IsExistingFile(artifactPath) {

			logger.Log(t, fmt.Sprintf("Test data artifacts located, checking for manifest file in "+
				"path: %s", artifactPath))
			manifest := &model.ContextTestManifest{}
			ts.LoadTestData(t, artifactPath, manifest)

			if manifest != nil && manifest.ModulesFolder != "" {

				var isResourceCleanupComplete = false
				if isCloudCleanRequested {
					contextConfig := readinessConfig.Contexts[artifact.Name()]
					tfOptions := CreateTerraformOptions(meta, readinessConfig, artifact.Name(),
						contextConfig, meta.DefaultConfigPath, path.Join(manifest.ModulesFolder, defaultTestSubFolder))
					isResourceCleanupComplete = Cleanup(t, meta, manifest.Name, &tfOptions)
				}

				if !isCloudCleanRequested || isResourceCleanupComplete {
					isSuccess = removeArtifactsAndFolders(t, meta, manifest)
					if !isSuccess {
						logger.Log(t, fmt.Sprintf("WARNING: failed to locate the test data "+
							"for artifact: %s in the /tmp folder", artifactPath))
					}
				}
			}
		}
	}
	return isSuccess
}

func removeArtifactsAndFolders(t *testing.T, meta model.ProvisionMeta, manifest *model.ContextTestManifest) bool {

	// Extra check, only removing /tmp/TestK8cSmoke*
	regex, err := regexp.Compile(defaultArtifactFormat)
	if err != nil || regex == nil {
		return false
	}

	if dir := regex.FindString(manifest.ModulesFolder); dir != "" {
		if meta.Enable.Simulate {
			logger.Log(t, fmt.Sprintf("SIMULATE removal of: %s", dir))
			return true
		}
		if err := os.RemoveAll(dir); err != nil {
			logger.Log(t, fmt.Sprintf("WARNING: failed to clean up at: %s err: %v", dir, err))
			return false

		}
		logger.Log(t, fmt.Sprintf("removed: %s", dir))
		return true
	}
	logger.Log(t, fmt.Sprintf("artifacts already removed as not existing: %s", manifest.ModulesFolder))
	return false
}

func removeManifestFolder(t *testing.T, meta model.ProvisionMeta) {

	regex, err := regexp.Compile(defaultParentArtifactFormat)
	if err == nil && regex != nil {
		artifactsRootDir := meta.ArtifactsRootDir
		if dir := regex.FindString(artifactsRootDir); dir != "" {
			if !meta.Enable.Simulate {

				if err := os.RemoveAll(artifactsRootDir); err != nil {
					logger.Log(t, fmt.Sprintf("WARNING: failed to clean up at: %s err: %v", artifactsRootDir, err))
				} else {
					logger.Log(t, fmt.Sprintf("removed parent test artifact: %s", meta.ArtifactsRootDir))
				}
			} else {
				logger.Log(t, fmt.Sprintf("SIMULATE removing parent test artifact: %s", artifactsRootDir))
			}
		}
	} else {
		logger.Log(t, fmt.Sprintf("WARNING: failed to locate the test parent artifact directory "+
			"to cleanup: %s in the /tmp folder due to error: %s", meta.ArtifactsRootDir, err.Error()))
	}
}
