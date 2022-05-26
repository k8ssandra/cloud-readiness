# K8ssandra Cloud Readiness Framework
## Usage Guide

## About

The following sections detail necessary activities required to customize a test using the cloud-readiness framework.  Configuration models are used to provision Kubernetes resources across various supported cloud providers.

Once the cloud environment is provisioned, the K8ssandra product can be installed/deployed, which utilizes foundational Kubernetes resources.

<span style="text-decoration:underline;">The cloud target environment is currently scoped to the Google Cloud Platform (GCP) for the initial release (v0.1) of the cloud-readiness framework.</span>

## Technologies
Technologies used for this solution include the following:
* Terraform
* Terratest
* GoLang
* Kubernetes
* Cloud APIs (GCP, AWS, Azure)
* Cloud readiness project artifacts

## Project layout
The [cloud-readiness project](https://github.com/k8ssandra/cloud-readiness) includes the **k8ssandra/provision** and **k8ssandra/test** folders.

* k8ssandra/provision 
  * Supports various cloud providers as Terraform artifacts. 
  * Sub-folders identify the supported cloud providers.
* k8ssandra/test 
  * Supports cloud-readiness framework utilities, configurations, test-data, and actual tests themselves.

Create a new folder under **k8ssandra/test/testdata/_&lt;your-scenario>_**.  

The folder is used to contain the scenario configuration and context files referenced by 1..N tests.


## Scenario creation

The following steps identify how to set up a test scenario from scratch.  Typically, an existing set of files can be copied as the foundation for a new scenario then adjusted accordingly.


### Step 1 - download the project

Clone the cloud-readiness framework repository to your local machine.


```
git clone https://github.com/k8ssandra/cloud-readiness
```


### Step 2 - define the contexts 

Create a new context file specific to your test scenario.  This file will describe network, cloud, and cluster context settings.  Think of this step as modeling out the topology of the cloud environment.

Copy an existing context file from the project, or create a new **context.go** file under the new **k8ssandra/testdata**/**_&lt;your-scenario>_** folder.  The contexts defined can be reused across other tests in future test runs.

First, create a **Contexts()** function with the signature defined below.  The package defined will need to be specific to the scenario name defined in step 1 as this will be referenced/reused by other tests.


#### Imports


```golang
package scenario_1

import (
    "github.com/k8ssandra/cloud-readiness/k8ssandra/test/model"
    "github.com/k8ssandra/cloud-readiness/k8ssandra/test/util"
)

func Contexts() map[string]model.ContextConfig { }

```


Inside the Contexts() function, the network definitions will be referenced.  In this case, two Traefik network configurations are defined, one for the** central** region and one for the **east **region.


#### Network model
Network specific settings for 3rd party ingress and egress rules as well as CIDR blocks.

```golang	
networkConfigCentral := model.NetworkConfig{
    TraefikValuesFile:   "k8c-traefik-bootz000.yaml",
    TraefikVersion:      util.DefaultTraefikVersion,
    SubnetCidrBlock:     "10.1.32.0/16",
    SecondaryCidrBlock:  "10.3.32.0/20",
    MasterIpv4CidrBlock: "10.0.0.0/21",
}

networkConfigEast := model.NetworkConfig{
    TraefikValuesFile:   "k8c-traefik-bootz001.yaml",
    TraefikVersion:      util.DefaultTraefikVersion,
    SubnetCidrBlock:     "10.2.32.0/16",
    SecondaryCidrBlock:  "10.4.32.0/20",
    MasterIpv4CidrBlock: "10.0.0.0/21",
}

```


After the network definitions, rack to zone assignments are made using the PoolRackConfig structure.  In this case, a rack name is assigned to each location/zone for the cloud region that will be defined in the CloudConfig. 

```golang
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
```



Once the racks are assigned, the cloud specific settings are defined. In this example, there is one for the central region and one for the east region.  Along with the regions there are settings for storage, machine type, and credentials.  These are used to provision the cloud environment.


#### Cloud model
The intent of this model is to specify settings specific to a particular cloud environment for a defined scenario.  This model will support multiple cloud providers in future versions. 

```golang
cloudConfigUsCentral := model.CloudConfig{
  Project:         "community-ecosystem",
  Region:          "us-central1",
  Locations:       []string{"us-central1-a"},
  PoolRackConfigs: centralRackConfigs,
  Environment:     "dev",
  MachineType:     "e2-standard-4",
  CredPath:        "<home-dir>.config/gcloud/application_default_credentials.json",
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
  CredPath:        "<home-dir>.config/gcloud/application_default_credentials.json",
  CredKey:         "GOOGLE_APPLICATION_CREDENTIALS",
  Bucket:          "google_storage_bucket",
}
```

Lastly, is the need to create contexts for modeling the cluster(s) to be provisioned.  These are the settings which uniquely define 1..* clusters in a cloud environment.  Notice that the previously defined network and regional cloud settings are referenced.


#### Contexts model

```golang
ctxConfig1 := model.ContextConfig {
    Name:          "bootz-c1",
    Namespace:     "bootz",
    CloudConfig:   cloudConfigUsCentral,
    ClusterLabels: []string{"control-plane", "data-plane"},
    NetworkConfig: networkConfigCentral,
}

ctxConfig2 := model.ContextConfig {
    Name:          "bootz-e1",
    Namespace:     "bootz",
    CloudConfig:   cloudConfigUsEast,
    ClusterLabels: []string{"data-plane"},
    NetworkConfig: networkConfigEast,
}

return map[string]model.ContextConfig {
    ctxConfig1.Name: ctxConfig1,
    ctxConfig2.Name: ctxConfig2,
}
```


### Step 3 - define the configurations
Create a customized configuration file to support the test scenario. This file will include infrastructure provisioning and K8ssandra installation details.  Again, this only needs to be created from scratch if unable to reuse an existing set of configurations.

Copy an existing config file, or create a new **readiness-config.go** file under the **testdata/_&lt;your-scenario>_** folder.  This configuration can be reused for future test runs.


#### Imports
```golang
package scenario_1

import (
    "github.com/gruntwork-io/terratest/modules/random"
    "github.com/k8ssandra/cloud-readiness/k8ssandra/test/model"
    "github.com/k8ssandra/cloud-readiness/k8ssandra/test/util"
    "strings"
    "testing"
)

func ReadinessConfig(t *testing.T, contexts map[string]model.ContextConfig) 
    (model.ProvisionMeta, model.ReadinessConfig) { }

```

#### Provision enablement model
In this case, the infrastructure provisioning is enabled. These are not mutually exclusive enablement flags and are designed to allow for simulation across removall, installation, infrastructure provisioning as well as pre-installation setup activities.  

```golang
var enablement = model.EnableConfig {
  Simulate:        false,
  RemoveAll:       false,
  Install:         false,
  ProvisionInfra:  true,
  PreInstallSetup: false,
}
```



#### Provision metadata model


```golang
var provisionMeta = model.ProvisionMeta {
  Enable:            enablement,
  ProvisionId:       "",
  ArtifactsRootDir:  "",
  KubeConfigs:       nil,
  ServiceAccount:    "",
  DefaultConfigPath: configPath,
  DefaultConfigDir:  configRootDir,
  AdminIdentity:     util.DefaultAdminIdentifier,
}
```



#### K8ssandra model
Specific to a K8ssandra installation (not infrastructure provisioning of cloud environment), this model provides installation details needed for the K8ssandra ecosystem.


```golang
k8cConfig := model.K8cConfig{
    ClusterName:             "bootz-k8c-cluster",
    ValuesFilePath:          "k8c-multi-dc.yaml", 
    ClusterScoped:           false,
}
```

#### Readiness model
Tying all the pieces together, the `ReadinessConfig` defines additional values necessary to provision and/or install K8ssandra.

```golang
func ReadinessConfig(t *testing.T, contexts map[string]model.ContextConfig) (model.ProvisionMeta, model.ReadinessConfig) {

  configRootDir, configPath := util.FetchKubeConfigPath(t)
	
  k8cConfig := model.K8cConfig {
    ClusterName:             "bootz-k8c-cluster",
    ValuesFilePath:          "k8c-multi-dc.yaml",
    MedusaSecretName:        "dev-k8ssandra-medusa-key",
    MedusaSecretFromFileKey: "medusa_gcp_key",
    MedusaSecretFromFile:    "medusa_gcp_key.json",
    ClusterScoped:            false,
   }

  tfConfig := model.TFConfig{
    ModuleFolder: "./provision/gcp",
  }

  helmConfig := model.HelmConfig{
    ChartPath: "k8ssandra/k8ssandra",
  }

  provisionConfig := model.ProvisionConfig{
    TFConfig:           tfConfig,
    HelmConfig:         helmConfig,
    K8cConfig:          k8cConfig,
    DefaultSleepSecs:   20,
    DefaultRetries:     30,
    DefaultTimeoutSecs: 240,
   }

  readinessConfig := model.ReadinessConfig {
    UniqueId:                 strings.ToLower(random.UniqueId()),
    Contexts:                 contexts,
    ServiceAccountNameSuffix: "sa",
    ExpectedNodeCount:        2,
    ProvisionConfig:          provisionConfig,
   }

return provisionMeta, readinessConfig
}
```



### Step 4 - create the test

Create a test .go file, name it anything that helps describe the test.  The only requirement is that it is suffixed with  “_test”

**Example**: 
k8c_smoke_test.go

Inside the test file, identify your test scenario folder in the import section.  Also, include the two other required imports.


```golang
import (
  . "github.com/k8ssandra/cloud-readiness/k8ssandra/test/testdata/scenario_1"
  . "github.com/k8ssandra/cloud-readiness/k8ssandra/test/util"
  "testing" 
)
```


Inside the test file, create a function beginning with the prefix “Test” name.  In this case, the test is named **TestK8cSmoke**.


```
func TestK8cSmoke(t *testing.T) { }
```


Creating the actual test wiring becomes super simple, allowing flexibility for extending the test case with post provisioning verifications.  Again, this can be copied and staged in the appropriate scenario folder to launch your test.


```golang
  meta, config := ReadinessConfig(t, Contexts())
  
  Apply(t, meta, config)
```


These two lines provide the following activities:



* Invoke the collection of contexts specific to a scenario.
* Construct a cloud-readiness model complete with metadata and configurations.
* Apply the desired activities based on the model and metadata.


## Cloud infrastructure provisioning

Setting the enablement configuration for provisioning.


```golang
var enablement = model.EnableConfig {
  Simulate:         false,
  RemoveAll:        false,
  Install:          false,
  ProvisionInfra:   true,
  PreInstallSetup:  false,
}
```


An alternative is to allow for pre-installation setup to occur after the cloud provisioning is completed for one or more clusters.

Currently, this includes a network (Traefik) and Cert-Manager installation for cluster-wide use.


```golang
var enablement = model.EnableConfig {
  Simulate:        false,
  RemoveAll:       false,
  Install:         false,
  ProvisionInfra:  true,
  PreInstallSetup: true,
}
```


Another alternative is to activate the provisioning and setup in a simulation mode.  This doesn’t apply the actual cloud provisioning, but rather allows the test executor to get log output indicating the model values, configurations, and steps that would be taken.


```golang
var enablement = model.EnableConfig {
  Simulate:         true,
  RemoveAll:        false,
  Install:          false,
  ProvisionInfra:   true,
  PreInstallSetup:  true,
}
```



## Test execution

Once the readiness model and enablement configurations are defined, a single command like the following will start the provisioning process.


```golang
go test -v -timeout 0 -p 3 k8c_smoke_test.go
```




* Supply **-v** for verbose output if desired.
* Supply a **-timeout** of zero to not have a timeout specified on the test run (unless you really want one).
* Supply **-p** for maximum number of tests to run simultaneously.  In a provisioning step this should match the number of clusters you want to provision.


## Cleanup
Post infrastructure provisioning, there will be provisioning and test artifacts available for reference.  Those can be removed as part of a provisioning model enablement.

```golang
var enablement = model.EnableConfig {
  Simulate:        false,
  RemoveAll:       true,
  Install:         false,
  ProvisionInfra:  true,
  PreInstallSetup: true,
}

var provisionMeta = model.ProvisionMeta {
  Enable:            enablement,
  ProvisionId:       "k8c-Qk9z7G",
  ArtifactsRootDir:  "/tmp/cloud-k8c-Qk9z7G",
  KubeConfigs:       nil,
  ServiceAccount:    "",
  DefaultConfigPath: configPath,
  DefaultConfigDir:  configRootDir,
  AdminIdentity:     util.DefaultAdminIdentifier,
}
```

If the `RemoveAll` flag is set to `true` and the `ProvisionId` along with its corresponding `ArtifactsRooDir` are defined, the process will remove all traces of the test artifacts.
Note: this is a somewhat manual way to do this at current, but the hooks are in place to perform the cleanup following a  smoke test verification in the future.

The same approach will apply to infrastructure provisioning clean as to the cleanup of the entire stack of K8ssandra resources.

## Todos



1. Version 0.1 baseline branch requires review.
2. A few open infrastructure and setup issues are remaining for the full e2e integration with k8ssandra testing.  One specific issue recently raised is the request to have a single network provisioned as opposed to scoping by cluster.
3. Version 0.1 baseline is scoped to the GCP/GKE cloud environment for provisioning and installation of the k8ssandra stack, however, 1..N cluster provisioning exists with initial deployment of the k8ssandra-operator release.
4. Once the GCP cloud-readiness functionality is reviewed and solidified, a blog post is planned for this scoped release.
5. Once the blog post is released for GCP, there is additional work to be applied for AWS and Azure to wire up  the provisioning infrastructure and IAM configurations.