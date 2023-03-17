# Changelog 0.5

## 0.5.0 - 2023-03-17

### Added

- Resync Theia Manager at startup. ([#150](https://github.com/antrea-io/theia/pull/150), [@yanjunz97])
  * Supports removing stale Spark jobs and database entries after restart.
  * Adding back queued Network Policy Recommendations job to sync list.
- Service Dependency Graph. ([#142](https://github.com/antrea-io/theia/pull/142), [@Dhruv-J])
  * Introduce sankey graph for visualizing traffic between pods and services.
- Traffic Drop Detection. ([#148](https://github.com/antrea-io/theia/pull/148), [@dreamtalen])
  * Adding a new Snowflake UDF to detect flows dropped or blocked by Network Policy.
- Throughput Anomaly Detection. ([#152](https://github.com/antrea-io/theia/pull/152), [@tushartathgur])
  * Adding new Spark job to detect anomalies in network flow and report threats
  * Introduce new CLI commands for starting Throughput Anomaly, check job status, and retrieve results
- Added new CLI `theia-sf version` & `theia version` to display theia version. ([#144](https://github.com/antrea-io/theia/pull/144), [@antoninbas])

### Changed

- Replaced the shell version of Clickhouse data migration to GoLang version ([#155](https://github.com/antrea-io/theia/pull/155), [@yanjunz97])
- Created a single Docker image for Policy Recommendation & Anomaly Detection ([#174](https://github.com/antrea-io/theia/pull/174), [@tushartathgur])

### Fixed

- Updating the getting started document to include Theia Manager ([#147](https://github.com/antrea-io/theia/pull/147), [@yanjunz97])


[@dreamtalen]: https://github.com/dreamtalen
[@yanjunz97]: https://github.com/yanjunz97
[@Dhruv-J]: https://github.com/Dhruv-J
[@antoninbas]: https://github.com/antoninbas
[@tushartathgur]: https://github.com/tushartathgur
