# Configuration models
Cloud-readiness provioning, installation, validations and 
cleanup are driven from the following models as part 
of test preconditions and execution.

 

### ProvisionConfig
Provision specific configuration providing generic 
properties as well as specific references to `Terraform`, `Helm`, 
`cloud` and `K8ssandra` configurations.  

```
PreTestCleanup     bool
PostTestCleanup    bool
CleanOnly          bool
CleanDir           string
DefaultRetries     int
DefaultSleepSecs   int
DefaultTimeoutSecs int
HelmConfig         HelmConfig
TFConfig           TFConfig
CloudConfig        CloudConfig
K8cConfig          K8cConfig
```

Referenced by the `ReadinessConfig`.

### ProvisionResult
Provisioning result feedback configuration.
```
Success bool
```

### ReadinessConfig
The primary configuration model used as starting point for test precondition setup and execution.

```
ProvisionConfig          ProvisionConfig
KubectlConfigPath        string
UniqueId                 string
RootFolder               string
ClusterNamePrefix        string
Contexts                 map[string]ContextConfig
ServiceAccountNamePrefix string
ExpectedNodeCount        int
```

### CloudConfig
Cloud specific configurations for `GCP`, `Azure`, and `AWS`.   

```
Type        string
Location    string
Region      string
Project     string
Name        string
CredPath    string
CredKey     string
Environment string
MachineType string
Bucket      string
```

Referenced by the `ProvisioningConfig`.

### TFConfig

Configurations for Terraform cloud-specific settings. Current support for `Google Cloud Storage` with 
modules defined supporting the following K8ssandra cloud resources:
* Storage buckets
* IAM
* VPC
* GKE



```
ModuleFolder string
```
__Future support for AWS and Azure modules in upcoming versions of cloud-readiness.__

More details [here](../../provision/gcp/env/README.md).

Referenced by the `ProvisioningConfig`.

### HelmConfig
Helm shared configurations for use when charts and Helm 
utilities are referenced.
```
ChartPath string
```
Referenced by the `ProvisioningConfig`.

### K8cConfig
K8ssandra specific configurations used for cloud-readiness 
test scenarios.
```
Version                 string
MedusaSecretName        string
MedusaSecretFromFileKey string
MedusaSecretFromFile    string
ValuesFilePath          string
ClusterScoped           bool
```
Referenced by the `ProvisioningConfig`.

### ContextConfig
Context configuration utilized by the `ReadinessConfig` 
for supporting 1..n contexts.

```
Name          string
Namespace     string
ClusterLabels []string
```