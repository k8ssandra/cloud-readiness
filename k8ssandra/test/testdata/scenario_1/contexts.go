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

package scenario_1

import (
	"github.com/k8ssandra/cloud-readiness/k8ssandra/test/model"
	"github.com/k8ssandra/cloud-readiness/k8ssandra/test/util"
)

func Contexts() map[string]model.ContextConfig {

	// Traefik
	networkConfigCentral := model.NetworkConfig{
		TraefikValuesFile: "k8c-traefik-bootz000.yaml",
		TraefikVersion:    util.DefaultTraefikVersion,
	}

	networkConfigEast := model.NetworkConfig{
		TraefikValuesFile: "k8c-traefik-bootz001.yaml",
		TraefikVersion:    util.DefaultTraefikVersion,
	}

	// Cloud specific
	cloudConfigUsCentral := model.CloudConfig{
		Project:     "community-ecosystem",
		Region:      "us-central1",
		Locations:   []string{"us-central1-a", "us-central1-b", "us-central1-c"},
		Environment: "dev",
		MachineType: "e2-standard-4",
		CredPath:    "/home/jbanks/.config/gcloud/application_default_credentials.json",
		CredKey:     "GOOGLE_APPLICATION_CREDENTIALS",
		Bucket:      "google_storage_bucket",
	}

	cloudConfigUsEast := model.CloudConfig{
		Project:     "community-ecosystem",
		Region:      "us-east1",
		Locations:   []string{"us-east1-b", "us-east1-c", "us-east1-d"},
		Environment: "dev",
		MachineType: "e2-standard-4",
		CredPath:    "/home/jbanks/.config/gcloud/application_default_credentials.json",
		CredKey:     "GOOGLE_APPLICATION_CREDENTIALS",
		Bucket:      "google_storage_bucket",
	}

	// Context scoping
	ctxConfig1 := model.ContextConfig{
		Name:          "bootz-c1",
		Namespace:     "bootz",
		CloudConfig:   cloudConfigUsCentral,
		ClusterLabels: []string{"control-plane", "data-plane"},
		NetworkConfig: networkConfigCentral,
	}

	ctxConfig2 := model.ContextConfig{
		Name:          "bootz-e1",
		Namespace:     "bootz",
		CloudConfig:   cloudConfigUsEast,
		ClusterLabels: []string{"data-plane"},
		NetworkConfig: networkConfigEast,
	}

	return map[string]model.ContextConfig{
		ctxConfig1.Name: ctxConfig1,
		ctxConfig2.Name: ctxConfig2,
	}

}
