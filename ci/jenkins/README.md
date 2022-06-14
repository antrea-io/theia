# Theia CI: Jenkins

## Reasons for Jenkins

We have tests as Github Actions but Jenkins allows tests running on a cluster of
multiple nodes and offers better environment setup options.

## List of Jobs

| Job Name | Description                                    | Trigger Phase   |
|----------|------------------------------------------------|-----------------|
|  theia-e2e-for-pull-request | Run e2e test for pull request                  | `/theia-test-e2e` |

## Requirements

Yaml files under [ci/jenkins/jobs](/ci/jenkins/jobs) can be generated via
jenkins-job-builder. If you want to try out the tests on your local jenkins
setup, please notice the following requirements:

* Jenkins setup
  * Plugins: ghprb, throttle-concurrents
* Install
  [jenkins-job-builder](https://docs.openstack.org/infra/jenkins-job-builder/index.html)
* Define your `ANTREA_GIT_CREDENTIAL` which is the credential for your private
  repo
* Define your `ghpr_auth`, `antrea_admin_list`, `antrea_org_list` and
  `antrea_white_list` as
  [defaults](https://docs.openstack.org/infra/jenkins-job-builder/definition.html#defaults)
  variables in a separate file

### Credentials

The following credentials on jenkins server should also be present for the jobs to work:

| Name         | Description                       |
|--------------|-----------------------------------|
| VMC_NETWORK_0 | vSphere Network name for VMs  |
 | VMC_NETWORK_1 | vSphere Network name for VMs  |
 | VCENTERNAME  | vCenter IP address                |
 | DATACENTERNAME | Datacenter name                   |
 | RESOURCEPOOLPATH | CI resource pool path             |
 | CAPVC_USERNAME | vCenter username              |
 | CAPVC_PASSWORD | vCenter password              |
| DATASTORE | Datastore name                    |
| VMFOLDERNAME | Folder name for VM                |
| THEIA_DOCKERHUB_USERNAME | Username for dockerhub, for image pull |
 | THEIA_DOCKERHUB_TOKEN | Login token for dockerhub, for image pull |

### Apply the jobs

Run the command to test if jobs can be generated correctly.  

```bash
jenkins-jobs test -r ci/jenkins/jobs
```

Run the command to apply these jobs.  

```bash
jenkins-jobs update -r ci/jenkins/jobs
```

## Tips for Developer

* [macro.yaml](/ci/jenkins/jobs/macros.yaml): Use "{{}}" instead of "{}" in your builder when you pass jenkins-job variable to the builder from projects.yaml
