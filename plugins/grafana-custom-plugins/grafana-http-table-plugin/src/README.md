<!-- This README file is going to be the one displayed on the Grafana.com website for your plugin. Uncomment and replace the content here before publishing.

Remove any remaining comments before publishing as these may be displayed on Grafana.com -->
# Grafana HTTP Table Plugin

## What is Grafana HTTP Table Plugin?

Grafana HTTP Table Plugin allows users to visualize the incoming HTTP values
that are encoded via JSON. Currently, the plugin shows the following HTTP
fields within a table: hostname, URL, user agent, content type, method,
protocol, status, and length.

## Acknowledgements

The Grafana HTTP Table Plugin is created using [MaterialTable](https://material-table.com/#/)

## Data Source

Supported Databases:

- Clickhouse

## Queries Convention

Currently, the HTTP Table Plugin is only for restricted use cases, and for
corrrect loading of data, the query is expected to return the following field:

- field 1: httpVals value with name or an alias of `httpVals`

ClickHouse query example:

```sql
SELECT httpVals
FROM flows
WHERE l7ProtocolName!=''
```

## Installation

### 1. Install the Panel

Installing on a local Grafana:

For local instances, plugins are installed and updated via a simple CLI command.
Use the grafana-cli tool to install chord-panel-plugin from the commandline:

```shell
grafana-cli --pluginUrl https://github.com/Dhruv-J/grafana-http-table-plugin/archive/refs/tags/v2.zip plugins install theia-grafana-dependency-plugin
```

The plugin will be installed into your grafana plugins directory; the default is
`/var/lib/grafana/plugins`. More information on the [cli tool](https://grafana.com/docs/grafana/latest/administration/cli/#plugins-commands).

Alternatively, you can manually download the .zip file and unpack it into your grafana
plugins directory.

[Download](https://github.com/Dhruv-J/grafana-http-table-plugin/archive/refs/tags/v2.zip)

Installing to a Grafana deployed on Kubernetes:

In Grafana deployment manifest, configure the environment variable `GF_INSTALL_PLUGINS`
as below:

```yaml
env:
- name: GF_INSTALL_PLUGINS
   value: "https://github.com/Dhruv-J/grafana-http-table-plugin/archive/refs/tags/v2.zip;theia-grafana-http-table-plugin"
```

### 2. Add the Panel to a Dashboard

Installed panels are available immediately in the Dashboards section in your Grafana
main menu, and can be added like any other core panel in Grafana. To see a list of
installed panels, click the Plugins item in the main menu. Both core panels and
installed panels will appear. For more information, visit the docs on [Grafana plugin installation](https://grafana.com/docs/grafana/latest/plugins/installation/).

## Customization

This plugin is built with [@grafana/plugin](https://grafana.com/developers/plugin-tools/),
which is a CLI that enables efficient development of Grafana plugins. To customize
the plugin and do local testings:

1. Install dependencies

   ```bash
   cd grafana-dependency-plugin
   yarn install
   ```

2. Build plugin in development mode or run in watch mode

   ```bash
   yarn dev
   ```

   or

   ```bash
   yarn watch
   ```

3. Build plugin in production mode

   ```bash
   yarn build
   ```

## Learn more

- [Build a panel plugin tutorial](https://grafana.com/tutorials/build-a-panel-plugin)
- [Grafana documentation](https://grafana.com/docs/)
- [Grafana Tutorials](https://grafana.com/tutorials/) - Grafana Tutorials are step-by-step
guides that help you make the most of Grafana
- [Grafana UI Library](https://developers.grafana.com/ui) - UI components to help you build interfaces using Grafana Design System
