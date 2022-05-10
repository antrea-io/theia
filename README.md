# Theia

Theia contains Network Flow Visibility functions which are exacted from the
[Antrea main repo](https://github.com/antrea-io/antrea). While flow exporter and
flow aggregator are kept in the Antrea repo, other flow visibility modules, such
as ClickHouse and Grafana related ones, will be moved to this new repo.

During the code migration period, we will still keep related functions available
and stable in the Antrea repo, and the functions in this new repo won't be ready
until we announce the migration is completed.
