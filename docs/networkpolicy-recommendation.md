# NetworkPolicy Recommendation

## Table of Contents

<!-- toc -->
- [Introduction](#introduction)
- [Installation](#installation)
- [Perform NetworkPolicy Recommendation](#perform-networkpolicy-recommendation)
  - [Run a policy recommendation job](#run-a-policy-recommendation-job)
  - [Check the status of a policy recommendation job](#check-the-status-of-a-policy-recommendation-job)
  - [Retrieve the result of a policy recommendation job](#retrieve-the-result-of-a-policy-recommendation-job)
<!-- /toc -->

## Introduction

Theia NetworkPolicy Recommendation recommends the NetworkPolicy configuration to secure Kubernetes
network and applications. It analyzes the network flows collected by
[Grafana Flow Collector](network-flow-visibility.md#grafana-flow-collector) to generate
[Kubernetes NetworkPolicies](https://kubernetes.io/docs/concepts/services-networking/network-policies/)
or [Antrea NetworkPolicies](https://github.com/antrea-io/antrea/blob/main/docs/antrea-network-policy.md).
This feature assists cluster administrators and app developers in securing their applications
according to Zero Trust principles.

## Installation

Theia's NetworkPolicy Recommendation capablity leverages Antrea Flow Exporter and Flow Aggreator.
Before leveraging this feature, follow this [doc](https://github.com/antrea-io/antrea/blob/main/docs/network-flow-visibility.md)
to ensure they are deployed. In addition, ensure `podLabels: true` is set in [Flow Aggregator Configuration](https://github.com/antrea-io/antrea/blob/main/docs/network-flow-visibility.md#configuration-1),
as Theia's NetworkPolicy Recommendation feature needs to process pod label information.

Then, please follow these [deployment steps](network-flow-visibility.md#deployment-steps)
to deploy Theia.

<!-- Comment out following lines since user won't need to deploy Spark Operator separately after PR#31 merged. -->

<!-- After we finish the deployment of Grafana Flow Collector and can see some flow records in the Grafana UI,
we can continue to deploy other necessary components of policy recommendation feature:

We need to install the [Kubernetes Operator for Apache Spark](https://github.com/GoogleCloudPlatform/spark-on-k8s-operator)
in the cluster. Our policy recommendation logic is implemented as a [Spark](https://github.com/apache/spark)
application, the Kubernetes Operator for Apache Spark helps us schedule the Spark job in the Kubernetes cluster.

Please run the following commands:

```bash
helm repo add spark-operator https://googlecloudplatform.github.io/spark-on-k8s-operator
helm install policy-reco spark-operator/spark-operator --namespace flow-visibility --set image.tag=latest
```

This will install the Kubernetes Operator for Apache Spark into the `flow-visibility` Namespace.

Once we finish these deployment steps, we should see the `policy-reco-spark-operator` Pod running inside the `flow-visibility` namespace:

```bash
$ kubectl get pods -n flow-visibility
NAME                                          READY   STATUS    RESTARTS   AGE
policy-reco-spark-operator-56c4cb454c-4vhfh   1/1     Running   0          2m19s
``` -->

## Perform NetworkPolicy Recommendation

Users can leverage Theia's NetworkPolicy recommendation feature through `theia` CLI.
`theia` is the command-line tool which provides access to Theia network flow visibility capabilities.
To get more information about `theia`, please refer to this [doc](theia.md).

There are 3 `theia` commands for the policy recommendation feature:

- `theia policy-recommendation run`
- `theia policy-recommendation status`
- `theia policy-recommendation retrieve`

Or you could use `pr` as a short alias of `policy-recommendation`:

- `theia pr run`
- `theia pr status`
- `theia pr retrieve`

To see all options and usage examples of these commands, you may run `theia policy-recommendation [subcommand] --help`.

### Run a policy recommendation job

The `theia policy-recommendation run` command triggers a new policy recommendation job.
If the new policy recommendation job is created successfully, the `recommendation ID` of this job will be returned:

```bash
$ theia policy-recommendation run
Successfully created policy recommendation job with ID e998433e-accb-4888-9fc8-06563f073e86
```

A policy recommendation job may take a few minutes to an hour to complete
depending on the number of network flows. To run a policy recommendation job
and wait until it is finished run:

```bash
theia policy-recommendation run --wait
```

### Check the status of a policy recommendation job

The `theia policy-recommendation status` command is used to check the status of a previous policy recommendation job.

For example the job we just created above, we could check its status via:

```bash
$ theia policy-recommendation status e998433e-accb-4888-9fc8-06563f073e86
Status of this policy recommendation job is COMPLETED
```

It will return the status of this policy recommendation job like `SUBMITTED`, `RUNNING`, `COMPLETED`, and `FAILED`.

For a complete list of the possible statuses of a policy recommendation job, please refer to the definition [here](https://github.com/GoogleCloudPlatform/spark-on-k8s-operator/blob/master/docs/api-docs.md#applicationstatetypestring-alias).

### Retrieve the result of a policy recommendation job

After a policy recommendation job completes, the recommended policies will be written into the Clickhouse database. We could use the `theia policy-recommendation retrieve` command to get the result:

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

To apply recommended policies in the cluster, we can save the recommended policies
to a yml file and apply it through `kubectl`:

```bash
theia policy-recommendation retrieve e998433e-accb-4888-9fc8-06563f073e86 -f recommended_policies.yml
kubectl apply -f recommended_policies.yml
```
