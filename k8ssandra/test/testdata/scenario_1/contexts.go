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

	// Network specific
	networkConfigCentral := model.NetworkConfig{
		TraefikValuesFile:   "k8c-traefik-bootz000.yaml",
		TraefikVersion:      util.DefaultTraefikVersion,
		SubnetCidrBlock:     "10.5.32.0/16",
		SecondaryCidrBlock:  "10.7.32.0/20",
		MasterIpv4CidrBlock: "10.0.0.0/21",
	}

	networkConfigEast := model.NetworkConfig{
		TraefikValuesFile:   "k8c-traefik-bootz001.yaml",
		TraefikVersion:      util.DefaultTraefikVersion,
		SubnetCidrBlock:     "10.6.32.0/16",
		SecondaryCidrBlock:  "10.8.32.0/20",
		MasterIpv4CidrBlock: "10.0.0.0/21",
	}

	centralRackConfigs := []model.PoolRackConfig{
		{
			Name:     "rack1",
			Label:    "k8ssandra.io/rack=rack1",
			Location: "us-central1-a",
		},
		{
			Name:     "rack2",
			Label:    "k8ssandra.io/rack=rack2",
			Location: "us-central1-b",
		},
		{
			Name:     "rack3",
			Label:    "k8ssandra.io/rack=rack3",
			Location: "us-central1-c",
		},
	}

	eastRackConfigs := []model.PoolRackConfig{
		{
			Name:     "rack1",
			Label:    "k8ssandra.io/rack=rack1",
			Location: "us-east1-b",
		},
		{
			Name:     "rack2",
			Label:    "k8ssandra.io/rack=rack2",
			Location: "us-east1-c",
		},
		{
			Name:     "rack3",
			Label:    "k8ssandra.io/rack=rack3",
			Location: "us-east1-d",
		},
	}

	// Cloud specific
	cloudConfigUsCentral := model.CloudConfig{
		Project:         "community-ecosystem",
		Region:          "us-central1",
		Locations:       []string{"us-central1-a"},
		PoolRackConfigs: centralRackConfigs,
		Environment:     "dev",
		MachineType:     "e2-standard-4",
		CredPath:        "/home/jbanks/.config/gcloud/application_default_credentials.json",
		CredKey:         "GOOGLE_APPLICATION_CREDENTIALS",
		Bucket:          "google_storage_bucket",
	}

	cloudConfigUsEast := model.CloudConfig{
		Project:         "community-ecosystem",
		Region:          "us-east1",
		Locations:       []string{"us-east1-b"},
		PoolRackConfigs: eastRackConfigs,
		Environment:     "dev",
		MachineType:     "e2-standard-4",
		CredPath:        "/home/jbanks/.config/gcloud/application_default_credentials.json",
		CredKey:         "GOOGLE_APPLICATION_CREDENTIALS",
		Bucket:          "google_storage_bucket",
	}

	// Context scoping
	ctxConfig1 := model.ContextConfig{
		Name:          "rio-c1walle100",
		Namespace:     "bootz",
		CloudConfig:   cloudConfigUsCentral,
		ClusterLabels: []string{"control-plane", "data-plane"},
		NetworkConfig: networkConfigCentral,
	}

	ctxConfig2 := model.ContextConfig{
		Name:          "rio-e1walle100",
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
