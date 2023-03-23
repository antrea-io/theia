# Changelog 0.3

## 0.3.0 - 2022-10-19

### Added

- Theia Manager. ([#97](https://github.com/antrea-io/theia/pull/97), [@wsquan171]) ([#109](https://github.com/antrea-io/theia/pull/109), [@wsquan171]) ([#111](https://github.com/antrea-io/theia/pull/111), [@yuntanghsu]) ([#113](https://github.com/antrea-io/theia/pull/113), [@wsquan171]) ([#114](https://github.com/antrea-io/theia/pull/114), [@yanjunz97]) ([#115](https://github.com/antrea-io/theia/pull/115), [@yuntanghsu])
  * The Theia Manager is a new component which provide logic for managing Theia operations, such as Network Policy Recommendation
  * Update policy recommendation and CLI to use Theia Manager
- Grafana UI enhancements. ([#101](https://github.com/antrea-io/theia/pull/101), [@heanlan]) ([#102](https://github.com/antrea-io/theia/pull/102), [@yuntanghsu])
  * Introduce a Grafana home dashboard
  * Improvement for throughput related panels in Grafana
- Support for Snowflake. ([#112](https://github.com/antrea-io/theia/pull/112), [@antoninbas]) ([#118](https://github.com/antrea-io/theia/pull/118), [@heanlan]) ([#119](https://github.com/antrea-io/theia/pull/119), [@heanlan])
  * Add support for Snowflake as a database backend for Theia
  * Enhance UI support for Snowflake

### Changed

- Adopt golang-migrate ClickHouse data schema management ([#95](https://github.com/antrea-io/theia/pull/95), [@yanjunz97])

[@antoninbas]: https://github.com/antoninbas
[@heanlan]: https://github.com/heanlan
[@wsquan171]: https://github.com/wsquan171
[@yanjunz97]: https://github.com/yanjunz97
[@yuntanghsu]: https://github.com/@yuntanghsu

