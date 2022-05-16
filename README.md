# K8ssandra cloud-readiness

The cloud-readiness framework is an open source _work-in-progress_ that provides decoupled management of 
infrastructure provisioning along with the installation and deployment of the [K8ssandra](https://github.com/k8ssandra/k8ssandra) stack 
in a cloud-specific environment.

Once provisioning and installation is complete, validations (e.g. tests) can be 
executed for proper inspections of a particular K8ssandra release in a specific cloud
environment. 

Following the validations, any installations, deployments, or infrastructure can be 
cleaned up based on the verification scenario defined. All of these activities will be controlled through a set of cloud-agnostic configuration models.

It was important that the cloud-readiness framework could be referenced locally in a Linux machine or as part of CI/CD pipeline.  The diagram below describes the framework's flexibilty of running
 on a user's local Linux or deployed to a centralized test environment.

![cloud-readiness-overview](https://github.com/k8ssandra/cloud-readiness/blob/main/docs/images/cloud-readiness-overview.svg)



## Platform provisioning
The cloud readiness framework provides cloud infrastructure provisioning to support test
environment setup and execution in the following cloud platforms:

* [GCP](https://github.com/k8ssandra/cloud-readiness/blob/main/k8ssandra/provision/gcp/env/README.md) - _in-progress_
* AWS - _coming soon ..._
* Azure - _coming soon ..._

Terraform modules are used for separation of cloud specific configurations.  As such, Terraform commands can be used as normal if needed as the framework exposes those stages of provisioning for maximum flexibility. 
This flexibility is important for troubleshooting needs or for development of a new version of cloud-readiness modules.


Test cases will utilize the Terraform modules for the cloud-specific platform setup and provisioning.  For example when needing a specific cluster, storage, IAM, network settings, etc.

To get started, checkout the repository and follow along in the cloud-readiness [user-guide](https://github.com/k8ssandra/cloud-readiness/blob/main/docs/user-guide.md) based on your GCP configuration.

Once the test file has your specific settings, issue `go test -v` from the smoke test folder to kickoff the provisioning activities.

## K8ssandra installation
The framework leverages a combination of open source technologies providing the user with
flexibility to install the [K8ssandra](https://github.com/k8ssandra/k8ssandra) stack, and target tests for execution in a cloud-specific environment.  

Following the test executions, the cleanup and teardown can be controlled based on the type of diagnostics needed.

All of these controls are exposed in the cloud-readiness [models](https://github.com/k8ssandra/cloud-readiness/blob/main/k8ssandra/test/model/model.go) and used as inputs for test run execution.  The models provide sharing and reuse for configurations used across verifications.

Stay tuned as the installation components evolve to support K8ssandra multi-cluster deployments.






