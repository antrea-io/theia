# Theia Command-line Tool

`theia` is the command-line tool which provides access to Theia network flow
visibility capabilities.

## Table of Contents

<!-- toc -->
- [Installation](#installation)
- [Usage](#usage)
  - [NetworkPolicy Recommendation feature](#networkpolicy-recommendation-feature)
  - [Throughput Anomaly Detection feature](#throughput-anomaly-detection-feature)
  - [ClickHouse](#clickhouse)
    - [Disk usage information](#disk-usage-information)
    - [Table Information](#table-information)
    - [Insertion rate](#insertion-rate)
    - [Stack trace](#stack-trace)
<!-- /toc -->

## Installation

`theia` binaries are published for different OS/CPU Architecture combionations.
For Linux, we also publish binaries for Arm-based systems. Refer to the
[releases page](https://github.com/antrea-io/theia/releases) and
download the appropriate one for your machine. For example:

On Mac & Linux:

```bash
curl -Lo ./theia "https://github.com/antrea-io/theia/releases/download/<TAG>/theia-$(uname)-x86_64"
chmod +x ./theia
mv ./theia /some-dir-in-your-PATH/theia
theia help
```

On Windows, using PowerShell:

```powershell
Invoke-WebRequest -Uri https://github.com/antrea-io/theia/releases/download/<TAG>/theia-windows-x86_64.exe -Outfile theia.exe
Move-Item .\theia.exe c:\some-dir-in-your-PATH\theia.exe
theia help
```

## Usage

To see the list of available commands and options, run `theia help`.

### NetworkPolicy Recommendation feature

We currently have 5 commands for NetworkPolicy Recommendation:

- `theia policy-recommendation run`
- `theia policy-recommendation status`
- `theia policy-recommendation retrieve`
- `theia policy-recommendation list`
- `theia policy-recommendation delete`

For details, please refer to [NetworkPolicy recommendation doc](
networkpolicy-recommendation.md)

### Throughput Anomaly Detection feature

We currently have 5 commands for Throughput Anomaly Detection:

- `theia throughput-anomaly-detection run`
- `theia throughput-anomaly-detection status`
- `theia throughput-anomaly-detection retrieve`
- `theia throughput-anomaly-detection list`
- `theia throughput-anomaly-detection delete`

For details, please refer to [Throughput Anomaly Detection doc](
throughput-anomaly-detection.md)

### ClickHouse

From Theia v0.2, we introduce one command for ClickHouse:

- `theia clickhouse status [flags]`

#### Disk usage information

The `--diskInfo` flag will list disk usage information of each ClickHouse shard. `Shard`, `DatabaseName`, `Path`, `Free`
, `Total` and `Used_Percentage`of each ClickHouse shard will be displayed in table format. For example:

```bash
$ theia clickhouse status --diskInfo
Shard          DatabaseName   Path                 Free           Total          Used_Percentage
1              default        /var/lib/clickhouse/ 1.84 GiB       1.84 GiB       0.04 %
```

#### Table Information

The `--tableInfo` flag will list basic table information of each ClickHouse shard. `Shard`, `DatabaseName`, `TableName`,
`TotalRows`, `TotalBytes` and `TotalCol`of tables in each ClickHouse shard will be displayed in table format. For example:

```bash
$ theia clickhouse status --tableInfo
Shard          DatabaseName   TableName                TotalRows      TotalBytes     TotalCols
1              default        .inner.flows_node_view   7              2.84 KiB       16
1              default        .inner.flows_pod_view    131            5.00 KiB       20
1              default        .inner.flows_policy_view 131            6.28 KiB       27
1              default        flows                    267            18.36 KiB      49
```

#### Insertion rate

The `--insertRate` flag will list the insertion rate of each ClickHouse shard. `Shard`, `RowsPerSecond`, and
`BytesPerSecond` of each ClickHouse shard will be displayed in table format. For example:

```bash
$ theia clickhouse status --insertRate
Shard          RowsPerSecond  BytesPerSecond
1              230            6.31 KiB
```

#### Stack trace

If ClickHouse is busy with something, and you don’t know what’s happening, you can check the stacktraces of all
the threads which are working.

The `--stackTraces` flag will list the stacktraces of each ClickHouse shard. `Shard`, `trace_function`, and
`count()` of each ClickHouse shard will be displayed in table format. For example:

```bash
$ theia clickhouse status --stackTraces
Row 1:
-------
Shard:           1
trace_functions: pthread_cond_timedwait@@GLIBC_2.3.2\nPoco::EventImpl::waitImpl(long)\nPoco::NotificationQueue::
waitDequeueNotification(long)\nDB::BackgroundSchedulePool::threadFunction()\n\nThreadPoolImpl<std::__1::thread>::
worker(std::__1::__list_iterator<std::__1::thread, void*>)\n\nstart_thread\n__clone
count():         128

Row 2:         
-------
Shard:           1
trace_functions: __poll\nPoco::Net::SocketImpl::pollImpl(Poco::Timespan&, int)\nPoco::Net::SocketImpl::poll(Poco::
Timespan const&, int)\nPoco::Net::TCPServer::run()\nPoco::ThreadImpl::runnableEntry(void*)\nstart_thread\n__clone
count():         5
```
