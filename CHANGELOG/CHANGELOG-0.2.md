# Changelog 0.2

## 0.2.0 - 2022-08-17

### Added

- ClickHouse cluster support and data schema management. ([#55](https://github.com/antrea-io/theia/pull/55), [@yanjunz97]) ([#81](https://github.com/antrea-io/theia/pull/81), [@yanjunz97])
  * Introduce support for ClickHouse cluster deployment with shards and replicas
  * Introduce version control to ClickHouse data schema, which detects the data schema version and applies necessary upgrades in initializing stage
- New Theia Command Line commands. ([#56](https://github.com/antrea-io/theia/pull/56), [@dreamtalen]) ([#59](https://github.com/antrea-io/theia/pull/59), [@yuntanghsu])
  * Commands for listing and deleting policy recommendation jobs
  * Commands to retrieve metrics in ClickHouse database

### Changed

- Enhancements to Helm chart based deployment ([#69](https://github.com/antrea-io/theia/pull/69), [@yanjun97]) ([#73](https://github.com/antrea-io/theia/pull/73), [@antoninbas])
- Move Grafana setting files to Persistent Volume ([#84](https://github.com/antrea-io/theia/pull/84), [@yanjun97])
- Grafana UI improvements ([#83](https://github.com/antrea-io/theia/pull/83), [@heanlan] [@dreamtalen] [@yuntanghsu])
- E2e tests for Theia functions. ([#66](https://github.com/antrea-io/theia/pull/66), [@yanjunz97]) ([#71](https://github.com/antrea-io/theia/pull/71), [@heanlan]) ([#77](https://github.com/antrea-io/theia/pull/77), [@dreamtalen])
  * Provide e2e tests for Theia components (ClickHouse monitor and Grafana dashboard)
  * Improve policy recommendation e2e test

[@antoninbas]: https://github.com/antoninbas
[@dreamtalen]: https://github.com/dreamtalen
[@heanlan]: https://github.com/heanlan
[@yanjunz97]: https://github.com/yanjunz97
[@yuntanghsu]: https://github.com/@yuntanghsu
