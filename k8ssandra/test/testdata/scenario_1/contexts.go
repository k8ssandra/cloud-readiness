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

	networkConfig000 := model.NetworkConfig{
		TraefikValuesFile: "k8c-traefik-bootz000.yaml",
		TraefikVersion:    util.DefaultTraefikVersion,
	}

	networkConfig001 := model.NetworkConfig{
		TraefikValuesFile: "k8c-traefik-bootz001.yaml",
		TraefikVersion:    util.DefaultTraefikVersion,
	}

	networkConfig002 := model.NetworkConfig{
		TraefikValuesFile: "k8c-traefik-bootz002.yaml",
		TraefikVersion:    util.DefaultTraefikVersion,
	}

	ctxConfig1 := model.ContextConfig{
		Name:          "bootz000",
		Namespace:     "bootz",
		ClusterLabels: []string{"control-plane"},
		NetworkConfig: networkConfig000,
	}

	ctxConfig2 := model.ContextConfig{
		Name:          "bootz001",
		Namespace:     "bootz",
		ClusterLabels: []string{"data-plane"},
		NetworkConfig: networkConfig001,
	}

	ctxConfig3 := model.ContextConfig{
		Name:          "bootz002",
		Namespace:     "bootz",
		ClusterLabels: []string{"data-plane"},
		NetworkConfig: networkConfig002,
	}

	return map[string]model.ContextConfig{
		ctxConfig1.Name: ctxConfig1,
		ctxConfig2.Name: ctxConfig2,
		ctxConfig3.Name: ctxConfig3,
	}

}
