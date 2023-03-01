# Throughput Anomaly Detector

## Table of Contents

<!-- toc -->
- [Introduction](#introduction)
- [Prerequisite](#prerequisite)
- [Perform Throughput Anomaly Detection](#perform-throughput-anomaly-detection)
  - [Run a throughput anomaly detection job](#run-a-throughput-anomaly-detection-job)
  - [Check the status of a throughput anomaly detection job](#check-the-status-of-a-throughput-anomaly-detection-job)
  - [Retrieve the result of a throughput anomaly detection job](#retrieve-the-result-of-a-throughput-anomaly-detection-job)
  - [List all throughput anomaly detection jobs](#list-all-throughput-anomaly-detection-jobs)
  - [Delete a throughput anomaly detection job](#delete-a-throughput-anomaly-detection-job)
<!-- /toc -->

## Introduction

From Theia v0.5, Theia supports Throughput Anomaly Detection.
Throughput Anomaly Detection (TAD) is a technique for understanding
and reporting the throughput abnormalities in the network traffic. It
analyzes the network flows collected by [Grafana Flow Collector](
network-flow-visibility.md#grafana-flow-collector) to report anomalies in the
network. TAD uses three algorithms to find the anomalies in network flows
such as ARIMA, EWMA, and DBSCAN. These anomaly analyses help the user to find
threats if present.

## Prerequisite

Please follow the [Getting Started](getting-started.md) guide to install Antrea
Flow Aggregator and Theia.

## Perform Throughput Anomaly Detection

Users can leverage Theia's Throughput Anomaly Detector feature through `theia`
CLI. `theia` is the command-line tool which provides access to Theia network
flow visibility capabilities. To get more information about `theia`, please
refer to its [user guide](theia-cli.md).

The following `theia` commands for the Throughput Anomaly Detector feature are
available:

- `theia throughput-anomaly-detection run`
- `theia throughput-anomaly-detection status`
- `theia throughput-anomaly-detection retrieve`
- `theia throughput-anomaly-detection list`
- `theia throughput-anomaly-detection delete`

Or you could use `tad` as a short alias of `throughput-anomaly-detection`:

- `theia tad run`
- `theia tad status`
- `theia tad retrieve`
- `theia tad list`
- `theia tad delete`

To see all options and usage examples of these commands, you may run
`theia throughput-anomaly-detection [subcommand] --help`.

### Run a throughput anomaly detection job

The `theia throughput-anomaly-detection run` command triggers an anomaly
detection job with a specific algorithm which can be provided with argument
`--algo`.
If a new throughput anomaly detection job is created successfully, the
`name` of this job will be returned:

```bash
$ theia throughput-anomaly-detection run --algo "ARIMA"
Successfully started Throughput Anomaly Detection job with name tad-1234abcd-1234-abcd-12ab-12345678abcd
```

The name of the Throughput Anomaly Detection job contains a universally
unique identifier ([UUID](
https://en.wikipedia.org/wiki/Universally_unique_identifier)) that is
automatically generated when creating a new throughput anomaly detection
job. We use this UUID to identify different throughput anomaly detection jobs.

By default, this command won't wait for the throughput anomaly detection
job to complete.

### Check the status of a throughput anomaly detection job

The `theia throughput-anomaly-detection status` command is used to check
the status of already created throughput anomaly detection job.

Given the job created above, we could check its status via:

```bash
$ theia throughput-anomaly-detection status tad-1234abcd-1234-abcd-12ab-12345678abcd
Status of this anomaly detection job is COMPLETED
```

It will return the status of this throughput anomaly detection job, which
can be one of `SUBMITTED`, `RUNNING`, `COMPLETED`, `FAILED`, etc.

For a complete list of the possible statuses of a throughput anomaly
detection job, please refer to the [doc](
https://github.com/GoogleCloudPlatform/spark-on-k8s-operator/blob/master/docs/api-docs.md#applicationstatetypestring-alias).

### Retrieve the result of a throughput anomaly detection job

After a throughput anomaly detection job completes, the anomalies detected
will be written into the ClickHouse database. The job will find all the
anomalies in the network. If the anomaly exists, it will be saved to
ClickHouse. Otherwise, the ClickHouse table will be updated with "NO
ANOMALY DETECTED" corresponding to the job name.

To retrieve results of the throughput anomaly detection job created above
in table format, run:

```bash
$ theia throughput-anomaly-detection retrieve tad-1234abcd-1234-abcd-12ab-12345678abcd
id                                      sourceIP        sourceTransportPort     destinationIP   destinationTransportPort        flowStartSeconds        flowEndSeconds          throughput                     algoCalc            anomaly
1234abcd-1234-abcd-12ab-12345678abcd    10.10.1.25      58076                   10.10.1.33      5201                            2022-08-11T06:26:54Z    2022-08-11 08:24:54     10004969097.000000000000000000  4.0063773860532994E9        true
1234abcd-1234-abcd-12ab-12345678abcd    10.10.1.25      58076                   10.10.1.33      5201                            2022-08-11T06:26:54Z    2022-08-11 08:06:54     4005703059.000000000000000000   1.0001208294655691E10       true
1234abcd-1234-abcd-12ab-12345678abcd    10.10.1.25      58076                   10.10.1.33      5201                            2022-08-11T06:26:54Z    2022-08-11 08:34:54     50007861276.000000000000000000  3.9735065921281104E9        true
```

User may also save the result in an output file in json format

### List all throughput anomaly detection jobs

The `theia throughput-anomaly-detection list` command lists all undeleted
throughput anomaly detection jobs. `CreationTime`, `CompletionTime`, `Name`
and `Status` of each throughput anomaly detection job will be displayed in
table format. For example:

```bash
$ theia throughput-anomaly-detection list
CreationTime          CompletionTime        Name                                    Status
2022-06-17 18:33:15   N/A                   tad-1234abcd-1234-abcd-12ab-12345678abcd RUNNING
2022-06-17 18:06:56   2022-06-17 18:08:37   tad-e998433e-accb-4888-9fc8-06563f073e86 COMPLETED
```

### Delete a throughput anomaly detection job

The `theia throughput-anomaly-detection delete` command is used to delete a
throughput anomaly detection job. This job would kill the process
corresponding to the job name irrespective of its status. This would also
delete all data related to the job name from ClickHouse DB. Please proceed
with caution since deletion cannot be undone. To delete the throughput
anomaly detection job created above, run:

```bash
$ theia throughput-anomaly-detection delete tad-1234abcd-1234-abcd-12ab-12345678abcd
Successfully deleted anomaly detection job with name: tad-1234abcd-1234-abcd-12ab-12345678abcd
```
