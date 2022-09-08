# Grafana Chord Panel Plugin

## What is Grafana Chord Panel Plugin?

Grafana Chord Panel Plugin enables users to create Chord Diagram panel in Grafana
Dashboards. It is used to visualize the network connections. Connections with
NetworkPolicy enforced will be highlighted. We use green color to highlight
the connection with "Allow" NetworkPolicy enforced, red color to highlight the
connection with "Deny" NetworkPolicy enforced. Panels are the building blocks
of Grafana. They allow you to visualize data in different ways. For more
information about panels, refer to the documentation on
[Panels](https://grafana.com/docs/grafana/latest/features/panels/panels/).
An example of Chord Panel Plugin:

<img src="https://downloads.antrea.io/static/05232022/chord-plugin-example.png" width="900" alt="Chord Panel Plugin Example">

## Acknowledgements

The Chord Plugin is created using [D3.js](https://d3js.org/).

## Data Source

Supported Databases:

- ClickHouse

## Queries Convention

Currently the Chord Plugin is created for restricted uses, only for visualizing
network flows between the source and destination, with enforced NetworkPolicy
metadata. For correct loading of the data for the Chord Plugin, the query is
expected to return the following fields, in arbitrary order.

- field 1: value to group by with name or an alias of `srcPod`
- field 2: value to group by with name or an alias of `srcPort`
- field 3: value to group by with name or an alias of `dstSvc`
- field 4: value to group by with name or an alias of `dstSvcPort`
- field 5: value to group by with name or an alias of `dstPod`
- field 6: value to group by with name or an alias of `dstPort`
- field 7: value to group by with name or an alias of `dstIP`
- field 8: the metric value field with an alias of `bytes`
- field 9: the metric value field with an alias of `revBytes`
- field 10: value to group by with name or an alias of `egressNetworkPolicyName`
- field 11: value to group by with name or an alias of `egressNetworkPolicyRuleAction`
- field 12: value to group by with name or an alias of `ingressNetworkPolicyName`
- field 13: value to group by with name or an alias of `ingressNetworkPolicyRuleAction`

Clickhouse query example:

```sql
SELECT sourcePodName as srcPod,
destinationPodName as dstPod,
sourceTransportPort as srcPort,
destinationTransportPort as dstPort,
destinationServicePort as dstSvcPort,
destinationServicePortName as dstSvc,
destinationIP as dstIP,
SUM(octetDeltaCount) as bytes,
SUM(reverseOctetDeltaCount) as revBytes,
egressNetworkPolicyName,
egressNetworkPolicyRuleAction,
ingressNetworkPolicyName,
ingressNetworkPolicyRuleAction
from flows
GROUP BY srcPod, dstPod, srcPort, dstPort, dstSvcPort, dstSvc, dstIP, egressNetworkPolicyName, egressNetworkPolicyRuleAction, ingressNetworkPolicyName, ingressNetworkPolicyRuleAction
```

## Installation

### 1. Install the Panel

Installing on a local Grafana:

For local instances, plugins are installed and updated via a simple CLI command.
Use the grafana-cli tool to install chord-panel-plugin from the commandline:

```shell
grafana-cli --pluginUrl https://downloads.antrea.io/artifacts/grafana-custom-plugins/theia-grafana-chord-plugin-1.0.0.zip plugins install theia-grafana-chord-plugin
```

The plugin will be installed into your grafana plugins directory; the default is
`/var/lib/grafana/plugins`. More information on the [cli tool](https://grafana.com/docs/grafana/latest/administration/cli/#plugins-commands).

Alternatively, you can manually download the .zip file and unpack it into your grafana
plugins directory.

[Download](https://downloads.antrea.io/artifacts/grafana-custom-plugins/theia-grafana-chord-plugin-1.0.0.zip)

Installing to a Grafana deployed on Kubernetes:

In Grafana deployment manifest, configure the environment variable `GF_INSTALL_PLUGINS`
as below:

```yaml
env:
- name: GF_INSTALL_PLUGINS
   value: "https://downloads.antrea.io/artifacts/grafana-custom-plugins/theia-grafana-chord-plugin-1.0.0.zip;theia-grafana-chord-plugin"
```

### 2. Add the Panel to a Dashboard

Installed panels are available immediately in the Dashboards section in your Grafana
main menu, and can be added like any other core panel in Grafana. To see a list of
installed panels, click the Plugins item in the main menu. Both core panels and
installed panels will appear. For more information, visit the docs on [Grafana plugin installation](https://grafana.com/docs/grafana/latest/plugins/installation/).

## Customization

This plugin is built with [@grafana/toolkit](https://www.npmjs.com/package/@grafana/toolkit),
which is a CLI that enables efficient development of Grafana plugins. To customize
the plugin and do local testings:

1. Install dependencies

   ```bash
   cd grafana-chord-plugin
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
