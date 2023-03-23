# Changelog 0.4

## 0.4.0 - 2022-12-23

### Added

- Support bundle collection. ([#140](https://github.com/antrea-io/theia/pull/140), [@wsquan171])
  * Add support bundle collection functionalities to Theia manager
- Structured policy recommendation results. ([#138](https://github.com/antrea-io/theia/pull/138), [@yanjunz97])
  * Update ClickHouse data schema to store each PR individually
  * Add PR results retrieving in the API server
- NetworkPolicy Recommendation application on the Snowflake backend. ([#137](https://github.com/antrea-io/theia/pull/137), [@dreamtalen])
  * Implement NetworkPolicy Recommendation as Snowflake UDFs which could be running on Snowflake warehouses
- Clickhouse statistic functions for Theia manager. ([#132](https://github.com/antrea-io/theia/pull/132), [@yuntanghsu])
  * Add the clickhouse statistic related functions for Theia Manager

[@dreamtalen]: https://github.com/dreamtalen
[@wsquan171]: https://github.com/wsquan171
[@yanjunz97]: https://github.com/yanjunz97
[@yuntanghsu]: https://github.com/yuntanghsu
