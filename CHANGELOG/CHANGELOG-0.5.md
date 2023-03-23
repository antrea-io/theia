# Changelog 0.5

## 0.5.0 - 2023-03-17

### Added

- Added Theia Manager resync at startup. ([#150](https://github.com/antrea-io/theia/pull/150), [@yanjunz97])
  * Support removing stale Spark jobs and database entries after restarting Theia Manager.
  * Scheduled Network Policy Recommendations jobs are added back to the periodical sync list.
- Added Service Dependency Graph. ([#142](https://github.com/antrea-io/theia/pull/142), [@Dhruv-J])
  * Introduced a directed graph for visualizing traffic between Pods and Services.
- Added new CLI `theia-sf version` & `theia version` to display theia version. ([#144](https://github.com/antrea-io/theia/pull/144), [@antoninbas]) ([#157](https://github.com/antrea-io/theia/pull/157), [@dreamtalen])

### Changed

- Replaced the shell version of Clickhouse data migration to the GoLang version ([#155](https://github.com/antrea-io/theia/pull/155), [@yanjunz97])
- Created a single Docker image for multiple Spark jobs ([#174](https://github.com/antrea-io/theia/pull/174), [@tushartathgur])

### Fixed

- Updated the getting started document to include Theia Manager ([#147](https://github.com/antrea-io/theia/pull/147), [@yanjunz97])


[@dreamtalen]: https://github.com/dreamtalen
[@yanjunz97]: https://github.com/yanjunz97
[@Dhruv-J]: https://github.com/Dhruv-J
[@antoninbas]: https://github.com/antoninbas
[@tushartathgur]: https://github.com/tushartathgur
