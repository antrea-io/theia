# Theia

Theia will contain Network Flow Visibility functions which are split from the
main repo. Flow exporter and flow aggregator will be kept in [Antrea](https://github.com/antrea-io/antrea)
repo, other modules, such as Clickhouse and Grafana related parts, will be moved
to this new repo.

During the code migration period, we will still keep related functions available
and stable in main repo, but the functions in this new repo won't be ready unless
we announce the migration is completed.
