package smoke

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
	. "github.com/k8ssandra/cloud-readiness/k8ssandra/test/testdata/scenario_1"
	. "github.com/k8ssandra/cloud-readiness/k8ssandra/test/util"
	"testing"
)

func TestK8cSmoke(t *testing.T) {
	meta, config := ReadinessConfig(t, Contexts())
	Apply(t, meta, config)
}
