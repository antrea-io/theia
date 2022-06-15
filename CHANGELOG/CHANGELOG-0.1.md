# Changelog 0.1

## 0.1.0 - 2022-06-16

This is the first Theia release and it includes every Antrea flow visibility capability released as of 1.6.

### Added

- Helm support for Theia installation. ([#15](https://github.com/antrea-io/theia/pull/15), [@yanjunz97]) ([#31](https://github.com/antrea-io/theia/pull/31), [@yanjunz97]) ([#34](https://github.com/antrea-io/theia/pull/34), [@yanjunz97])
  * Provide Helm charts for Theia components (Spark Operator, Clickhouse, and Grafana)
  * Allow for deploying Clickhouse with a persistent volume storage
- Grafana dashboard: add a panel for visualizing denied connections. ([#12](https://github.com/antrea-io/theia/pull/12), [@heanlan])
  * Introduce chord diagram for visualizing connections denied by network policies
  * Reorganized Network policy dashboard to visualize allowed, denied, and unprotected traffic
- Policy recommendation via Spark job. ([#16](https://github.com/antrea-io/theia/pull/16), [@dreamtalen])
  * Add a spark job to perform network policy recommendation based on flow data pushed by Flow Aggregator
- Theia Command Line Interface. ([#25](https://github.com/antrea-io/theia/pull/25), [@dreamtalen] [@yanjunz97] [@annakhm])
  * Add `theia` CLI tool to provide access to Theia network flow visibility capabilities
  * Introduce commands for starting Network Policy Recommendation, check job status, and retrieve results
- Initial Theia Documentation. ([#40](https://github.com/antrea-io/theia/pull/45/), [@dreamtalen]) ([#45](https://github.com/antrea-io/theia/pull/45/), [@jianjuns])

### Changed

- Add a limit to the quantify of data rendered by Grafana dashboard to avoid bad performance, add indirect cycle handling in Sankey panel ([#42](https://github.com/antrea-io/theia/pull/42), [@heanlan])

[@annakhm]: https://github.com/annakhm
[@dreamtalen]: https://github.com/dreamtalen
[@hangyan]: https://github.com/hangyan
[@heanlan]: https://github.com/heanlan
[@jianjuns]: https://github.com/jianjuns
[@yanjunz97]: https://github.com/yanjunz97
