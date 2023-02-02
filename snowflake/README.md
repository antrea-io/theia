# Theia with Snowflake

## Table of Contents

<!-- toc -->
- [Overview](#overview)
- [Getting started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [Install the theia-sf CLI](#install-the-theia-sf-cli)
  - [Configure AWS credentials](#configure-aws-credentials)
  - [Configure Snowflake credentials](#configure-snowflake-credentials)
  - [Create an S3 bucket to store infrastructure state](#create-an-s3-bucket-to-store-infrastructure-state)
  - [Create a KMS key to encrypt infrastructure state](#create-a-kms-key-to-encrypt-infrastructure-state)
  - [Provision all cloud resources](#provision-all-cloud-resources)
  - [Configure the Flow Aggregator in your cluster(s)](#configure-the-flow-aggregator-in-your-clusters)
- [Clean up](#clean-up)
- [Running applications](#running-applications)
  - [NetworkPolicy Recommendation](#networkpolicy-recommendation)
  - [Abnormal Traffic Drop Detector](#abnormal-traffic-drop-detector)
- [Network flow visibility with Grafana](#network-flow-visibility-with-grafana)
  - [Configure datasource](#configure-datasource)
  - [Deployments](#deployments)
  - [Pre-built dashboards](#pre-built-dashboards)
    - [View flow data from selected cluster(s)](#view-flow-data-from-selected-clusters)
<!-- /toc -->

## Overview

We are introducing the ability to use Snowflake as the storage and computing
platform for Theia. When using Snowflake, it is no longer necessary to run
ClickHouse DB (flow records are stored in a Snowflake database) or Spark (flow
processing is done by Snowflake virtual warehouses). Using Snowflake for Theia
means that you have to bring your own AWS and Snowflake accounts (you will be
charged for resource usage) and some features available with "standard" Theia
are not available yet with Snowflake.

## Getting started

### Prerequisites

Theia with Snowflake requires Antrea >= v1.9.0 and Theia >= v0.3.0.

### Install the theia-sf CLI

Because it is not yet distributed as a release asset, you will need to build the
CLI yourself, which requires Git, Golang and Make.

```bash
git clone git@github.com:antrea-io/theia.git
cd theia/snowflake
make
```

### Configure AWS credentials

Follow the steps in the [AWS
documentation](https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/#specifying-credentials)
to specify credentials that can be used by the AWS Go SDK. Either configure the
~/.aws/credentials file or set the required environment variables.

You can also export the `AWS_REGION` environment variable, set to your preferred
region.

### Configure Snowflake credentials

Export the following environment variables: `SNOWFLAKE_ACCOUNT`,
`SNOWFLAKE_USER`, `SNOWFLAKE_PASSWORD`.

### Create an S3 bucket to store infrastructure state

You may skip this step if you already have a bucket that you want to use.

```bash
./bin/theia-sf create-bucket
```

Retrieve the bucket name output by the command.

### Create a KMS key to encrypt infrastructure state

You may skip this step if you already have a KMS key that you want to use. If
you choose *not* to use a KMS key, Snowflake credentials will be stored in your
S3 bucket in clear text.

```bash
./bin/theia-sf create-kms-key
```

Retrieve the key ID output by the command.

### Provision all cloud resources

```bash
./bin/theia-sf onboard --bucket-name <BUCKET NAME> --key-id <KEY ID>
```

The command output will include a table like this one, with important information:

```text
+----------------------------+------------------------------------------------------------------+
| Region                     | us-west-2                                                        |
| Bucket Name                | antrea-flows-93e9ojn80fgn5vwt                                    |
| Bucket Flows Folder        | flows                                                            |
| Snowflake Database Name    | ANTREA_93E9OJN80FGN5VWT                                          |
| Snowflake Schema Name      | THEIA                                                            |
| Snowflake Flows Table Name | FLOWS                                                            |
| SNS Topic ARN              | arn:aws:sns:us-west-2:867393676014:antrea-flows-93e9ojn80fgn5vwt |
| SQS Queue ARN              | arn:aws:sqs:us-west-2:867393676014:antrea-flows-93e9ojn80fgn5vwt |
+----------------------------+------------------------------------------------------------------+
```

### Configure the Flow Aggregator in your cluster(s)

```bash
helm repo add antrea https://charts.antrea.io
helm repo update
helm install antrea antrea/antrea -n kube-system --set featureGates.FlowExporter=true
helm install flow-aggregator antrea/flow-aggregator \
     --set s3Uploader.enable=true \
     --set s3Uploader.bucketName=<BUCKET NAME> \
     --set s3Uploader.bucketPrefix=flows \
     --set s3Uploader.awsCredentials.aws_access_key_id=<AWS ACCESS KEY ID> \
     --set s3Uploader.awsCredentials.aws_secret_access_key=<AWS SECRET ACCESS KEY> \
     -n flow-aggregator --create-namespace
```

## Clean up

Follow these steps if you want to delete all resources created by
`theia-sf`. Just like for [onboarding](#getting-started), AWS credentials and
Snowflake credentials are required.

```bash
# always call offboard with the same arguments as onboard!
./bin/theia-sf offboard --bucket-name <BUCKET NAME> --key-id <KEY ID>
# if you created a KMS key and you want to schedule it for deletion
./bin/theia-sf delete-kms-key --key-id <KEY ID>
# if you created an S3 bucket to store infra state and you want to delete it
./bin/theia-sf delete-bucket --name <BUCKET NAME>
```

## Running applications

### NetworkPolicy Recommendation

NetworkPolicy Recommendation recommends the NetworkPolicy configuration
to secure Kubernetes network and applications. It analyzes the network flows
stored in the Snowflake database to generate
[Kubernetes NetworkPolicies](
https://kubernetes.io/docs/concepts/services-networking/network-policies/)
or [Antrea NetworkPolicies](
https://github.com/antrea-io/antrea/blob/main/docs/antrea-network-policy.md).

```bash
# make sure you have called onboard before running policy-recommendation
./bin/theia-sf policy-recommendation --database-name <SNOWFLAKE DATABASE NAME> > recommended_policies.yml
```

Database name can be found in the output of the [onboard](#getting-started)
command.

NetworkPolicy Recommendation requires a Snowflake warehouse to execute and may
take seconds to minutes depending on the number of flows. We recommend using a
[Medium size warehouse](https://docs.snowflake.com/en/user-guide/warehouses-overview.html)
if you are working on a big dataset. If no warehouse is provided by the
`--warehouse-name` option, we will create a temporary X-Small size warehouse by
default. Running NetworkPolicy Recommendation will consume Snowflake credits,
the amount of which will depend on the size of the warehouse and the contents
of the database.

### Abnormal Traffic Drop Detector

The Abnormal Traffic Drop Detector scans flow records stored in the database
and detects any unreasonable amount of flows dropped or blocked by
NetworkPolicy for each endpoint, then reports alerts to users. It helps
identify potential issues with NetworkPolicies and potential security threats.
Also, it could alert admins to take appropriate action to mitigate them.

```bash
# make sure you have called onboard before running drop-detection
./bin/theia-sf drop-detection --database-name <SNOWFLAKE DATABASE NAME>
```

Database name can be found in the output of the [onboard](#getting-started)
command.

Just like for NetworkPolicy Recommendation, the Abnormal Traffic Drop
Detector requires a Snowflake warehouse to execute and may take seconds to
minutes depending on the number of flows. If no warehouse is provided by the
`--warehouse-name` option, we will create a temporary X-Small size warehouse by
default. Running Abnormal Traffic Drop Detector will consume Snowflake credits,
the amount of which will depend on the size of the warehouse and the contents
of the database.

## Network flow visibility with Grafana

We use Grafana as the tool to query data from Snowflake, and visualize the
networking flows in the cluster(s).

### Configure datasource

Export the following environment variables:

 Name                     | Description
------------------------- | ------------
 SNOWFLAKE_ACCOUNT        | Specifies the full name of your account (provided by Snowflake).
 SNOWFLAKE_USER           | Specifies the login name of the user for the connection.
 SNOWFLAKE_PASSWORD       | Specifies the password for the specified user.
 SNOWFLAKE_WAREHOUSE      | Specifies the virtual warehouse to use once connected.
 SNOWFLAKE_DATABASE       | Specifies the default database to use once connected.
 SNOWFLAKE_ROLE (Optional)| Specifies the default access control role to use in the Snowflake session initiated by Grafana.

Database name can be found in the output of the [onboard](#getting-started)
command.

### Deployments

We suggest running Grafana using the official Docker image.

Before running Grafana, we need to download the Snowflake datasource plugin.
We suggest creating your own plugin directory.

```bash
mkdir your-plugin-path && cd your-plugin-path
wget https://github.com/michelin/snowflake-grafana-datasource/releases/download/v1.2.0/snowflake-grafana-datasource.zip
unzip snowflake-grafana-datasource.zip
export GF_PLUGINS=$(pwd)
```

Then run Grafana with Docker:

```bash
cd theia/snowflake

docker run -d \
-p 3000:3000 \
-v "${GF_PLUGINS}":/var/lib/grafana/plugins \
-v "$(pwd)"/grafana/provisioning:/etc/grafana/provisioning \
--name=grafana \
-e "GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS=michelin-snowflake-datasource" \
-e "GF_INSTALL_PLUGINS=https://downloads.antrea.io/artifacts/grafana-custom-plugins/theia-grafana-sankey-plugin-1.0.2.zip;theia-grafana-sankey-plugin,https://downloads.antrea.io/artifacts/grafana-custom-plugins/theia-grafana-chord-plugin-1.0.1.zip;theia-grafana-chord-plugin" \
-e "GF_DASHBOARDS_DEFAULT_HOME_DASHBOARD_PATH=/etc/grafana/provisioning/dashboards/homepage.json" \
-e "SNOWFLAKE_ACCOUNT=${SNOWFLAKE_ACCOUNT}" \
-e "SNOWFLAKE_USER=${SNOWFLAKE_USER}" \
-e "SNOWFLAKE_PASSWORD=${SNOWFLAKE_PASSWORD}" \
-e "SNOWFLAKE_WAREHOUSE=${SNOWFLAKE_WAREHOUSE}" \
-e "SNOWFLAKE_DATABASE=${SNOWFLAKE_DATABASE}" \
-e "SNOWFLAKE_ROLE=${SNOWFLAKE_ROLE}" \
grafana/grafana:9.1.6
```

Open your web browser and go to `http://localhost:3000/`, login with:

- username: admin
- password: admin

### Pre-built dashboards

You will see the home dashboard after login. It provides an overview
of the monitored Kubernetes cluster, short introduction and links
to the other pre-built dashboards for more specific flow visualization.
Detailed introduction of these dashboards can be found at
[Pre-built Dashboards](https://github.com/antrea-io/theia/blob/main/docs/network-flow-visibility.md#pre-built-dashboards).

Currently, with Snowflake datasource, we have built the Home Dashboard, Flow
Records Dashboard, Pod-to-Pod Flows Dashboard, and Network-Policy Flows
Dashboard.

#### View flow data from selected cluster(s)

The dashboards allow you to select and view flow data from one or more
clusters. You will be able to find a dropdown menu on the top left corner of
each dashboard. The filter will affect all the panels in the dashboard.

<img src="https://downloads.antrea.io/static/10052022/clusterid-filter.png" width="900" alt="Filtering on clusterID">
