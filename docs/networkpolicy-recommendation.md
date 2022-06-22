# NetworkPolicy Recommendation

## Table of Contents

<!-- toc -->
- [Introduction](#introduction)
- [Prerequisite](#prerequisite)
- [Perform NetworkPolicy Recommendation](#perform-networkpolicy-recommendation)
  - [Run a policy recommendation job](#run-a-policy-recommendation-job)
  - [Check the status of a policy recommendation job](#check-the-status-of-a-policy-recommendation-job)
  - [Retrieve the result of a policy recommendation job](#retrieve-the-result-of-a-policy-recommendation-job)
  - [List all policy recommendation jobs](#list-all-policy-recommendation-jobs)
  - [Delete a policy recommendation job](#delete-a-policy-recommendation-job)
<!-- /toc -->

## Introduction

Theia NetworkPolicy Recommendation recommends the NetworkPolicy configuration
to secure Kubernetes network and applications. It analyzes the network flows
collected by [Grafana Flow Collector](
network-flow-visibility.md#grafana-flow-collector) to generate
[Kubernetes NetworkPolicies](
https://kubernetes.io/docs/concepts/services-networking/network-policies/)
or [Antrea NetworkPolicies](
https://github.com/antrea-io/antrea/blob/main/docs/antrea-network-policy.md).
This feature assists cluster administrators and app developers in securing
their applications according to Zero Trust principles.

## Prerequisite

Please follow the [Getting Started](getting-started.md) guide to install Antrea
Flow Aggregator and Theia.

## Perform NetworkPolicy Recommendation

Users can leverage Theia's NetworkPolicy Recommendation feature through `theia`
CLI. `theia` is the command-line tool which provides access to Theia network
flow visibility capabilities. To get more information about `theia`, please
refer to its [user guide](theia-cli.md).

The following `theia` commands for the NetworkPolicy Recommendation feature are
available:

- `theia policy-recommendation run`
- `theia policy-recommendation status`
- `theia policy-recommendation retrieve`
- `theia policy-recommendation list`
- `theia policy-recommendation delete`

Or you could use `pr` as a short alias of `policy-recommendation`:

- `theia pr run`
- `theia pr status`
- `theia pr retrieve`
- `theia pr list`
- `theia pr delete`

To see all options and usage examples of these commands, you may run
`theia policy-recommendation [subcommand] --help`.

### Run a policy recommendation job

The `theia policy-recommendation run` command triggers a new policy
recommendation job.
If a new policy recommendation job is created successfully, the
`recommendation ID` of this job will be returned:

```bash
$ theia policy-recommendation run
Successfully created policy recommendation job with ID e998433e-accb-4888-9fc8-06563f073e86
```

`recommendation ID` is a universally unique identifier ([UUID](
https://en.wikipedia.org/wiki/Universally_unique_identifier)) that is
automatically generated when creating a new policy recommendation job. We use
`recommendation ID` to identify different policy recommendation jobs.

A policy recommendation job may take a few minutes to more than an hour to
complete depending on the number of network flows. By default, this command
won't wait for the policy recommendation job to complete. If you would like to
wait until the job is finished, add an option `--wait` at the end of command:

```bash
theia policy-recommendation run --wait
```

### Check the status of a policy recommendation job

The `theia policy-recommendation status` command is used to check the status of
a previous policy recommendation job.

Given the job created above, we could check its status via:

```bash
$ theia policy-recommendation status e998433e-accb-4888-9fc8-06563f073e86
Status of this policy recommendation job is COMPLETED
```

It will return the status of this policy recommendation job, which can be one
of `SUBMITTED`, `RUNNING`, `COMPLETED`, `FAILED`, etc.

For a complete list of the possible statuses of a policy recommendation job,
please refer to the [doc](
https://github.com/GoogleCloudPlatform/spark-on-k8s-operator/blob/master/docs/api-docs.md#applicationstatetypestring-alias).

### Retrieve the result of a policy recommendation job

After a policy recommendation job completes, the recommended policies will be
written into the Clickhouse database. To retrieve results of the policy
recommendation job created above, run:

```bash
$ theia policy-recommendation retrieve e998433e-accb-4888-9fc8-06563f073e86
apiVersion: crd.antrea.io/v1alpha1
kind: ClusterNetworkPolicy
metadata:
name: recommend-allow-acnp-kube-system-q7loe
spec:
appliedTo:
- namespaceSelector:
    matchLabels:
        kubernetes.io/metadata.name: kube-system
egress:
- action: Allow
    to:
    - podSelector: {}
ingress:
- action: Allow
    from:
    - podSelector: {}
priority: 5
tier: Platform
---
... other policies
```

To apply recommended policies in the cluster, we can save the recommended
policies to a YAML file and apply it using `kubectl`:

```bash
theia policy-recommendation retrieve e998433e-accb-4888-9fc8-06563f073e86 -f recommended_policies.yml
kubectl apply -f recommended_policies.yml
```

### List all policy recommendation jobs

The `theia policy-recommendation list` command lists all undeleted policy
recommendation jobs. `CreationTime`, `CompletionTime`, `ID` and `Status` of each
policy recommendation job will be displayed in table format. For example:

```bash
> theia policy-recommendation list
CreationTime          CompletionTime        ID                                   Status
2022-06-17 18:33:15   N/A                   2cf13427-cbe5-454c-b9d3-e1124af7baa2 RUNNING
2022-06-17 18:06:56   2022-06-17 18:08:37   e998433e-accb-4888-9fc8-06563f073e86 COMPLETED
```

### Delete a policy recommendation job

The `theia policy-recommendation delete` command is used to delete a policy
recommendation job. Please proceed with caution since deletion cannot be
undone. To delete the policy recommendation job created above, run:

```bash
$ theia policy-recommendation delete e998433e-accb-4888-9fc8-06563f073e86
Successfully deleted policy recommendation job with ID e998433e-accb-4888-9fc8-06563f073e86
```
