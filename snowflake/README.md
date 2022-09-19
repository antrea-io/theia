# Theia with Snowflake

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

```
helm repo add antrea https://charts.antrea.io
helm repo update
helm install antrea antrea/antrea -n kube-system --set featureGates.FlowExporter=true
helm install flow-aggregator antrea/flow-aggregator \
     --set s3Uploader.enable=true \
     --set s3Uploader.bucketName=<BUCKET NAME> \
     --set s3Uploader.bucketName=flows \
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

We are in the process of adding support for applications to Snowflake-powered
Theia, starting with NetworkPolicy recommendation.
