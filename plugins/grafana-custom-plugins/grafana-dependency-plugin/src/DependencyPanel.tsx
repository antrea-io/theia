import React from 'react';
import mermaid from 'mermaid';
import { DependencyOptions } from 'types';
import { PanelProps } from '@grafana/data';
import { useTheme2 } from '@grafana/ui';
import { DiagramPanelController } from 'DiagramController';

interface Props extends PanelProps<DependencyOptions> {}

export const DependencyPanel: React.FC<Props> = ({ options, data, width, height }) => {
  const theme = useTheme2();
  const frame = data.series[0];
  const sourcePodNames = frame.fields.find((field) => field.name === 'sourcePodName');
  const sourcePodLabels = frame.fields.find((field) => field.name === 'sourcePodLabels');
  const sourceNodeNames = frame.fields.find((field) => field.name === 'sourceNodeName');
  const destinationPodNames = frame.fields.find((field) => field.name === 'destinationPodName');
  const destinationPodLabels = frame.fields.find((field) => field.name === 'destinationPodLabels');
  const destinationNodeNames = frame.fields.find((field) => field.name === 'destinationNodeName');
  const destinationServicePortNames = frame.fields.find((field) => field.name === 'destinationServicePortName');
  const octetDeltaCounts = frame.fields.find((field) => field.name === 'octetDeltaCount');
  const sourceTransportPorts = frame.fields.find((field) => field.name === 'sourceTransportPort');
  const destinationTransportPorts = frame.fields.find((field) => field.name === 'destinationTransportPort');
  const sourceIPs = frame.fields.find((field) => field.name === 'sourceIP');
  const destinationIPs = frame.fields.find((field) => field.name === 'destinationIP');
  const httpValsList = frame.fields.find((field) => field.name === 'httpVals');
  
  let nodeToPodMap = new Map<string, String[]>();
  let srcToDestMap = new Map<string, Map<string, number>>();

  let graphString = 'graph LR;\n';
  let styleString = '';
  let boxColor;
  let layerLevel;

  if (options.layerFour) {
    layerLevel = 'four';
  } else {
    layerLevel = 'seven';
  }

  switch(options.color) {
    case 'red':
      boxColor = theme.colors.error.main;
      break;
    case 'yellow':
      boxColor = theme.colors.warning.main;
      break;
    case 'green':
      boxColor = theme.colors.success.main;
      break;
    case 'blue':
      boxColor = theme.colors.primary.main;
      break;
  }

  function layerFourGraph() {
    for (let i = 0; i < frame.length; i++) {
      const sourcePodName = sourcePodNames?.values.get(i);
      const sourcePodLabel = sourcePodLabels?.values.get(i);
      const sourceNodeName = sourceNodeNames?.values.get(i);
      const destinationPodName = destinationPodNames?.values.get(i);
      const destinationPodLabel = destinationPodLabels?.values.get(i);
      const destinationNodeName = destinationNodeNames?.values.get(i);
      const destinationServicePortName = destinationServicePortNames?.values.get(i);
      const octetDeltaCount = octetDeltaCounts?.values.get(i);
  
      function getName(groupByLabel: boolean, source: boolean, labelJSON: string) {
        if(!groupByLabel || labelJSON === undefined || options.labelName === undefined) {
          return source ? sourcePodName : destinationPodName;
        }
        let labels = JSON.parse(labelJSON);
        if(labels[options.labelName] !== undefined) {
          return labels[options.labelName];
        }
        return sourcePodName;
      }
  
      let groupByPodLabel = options.groupByPodLabel;
      let srcName = getName(groupByPodLabel, true, sourcePodLabel);
      let dstName = getName(groupByPodLabel, false, destinationPodLabel);
  
      // determine which nodes contain which pods
      if (nodeToPodMap.has(sourceNodeName) && !nodeToPodMap.get(sourceNodeName)?.includes(srcName)) {
        nodeToPodMap.get(sourceNodeName)?.push(srcName);
      } else if (!nodeToPodMap.has(sourceNodeName)) {
        nodeToPodMap.set(sourceNodeName, [srcName]);
      }
      if (nodeToPodMap.has(destinationNodeName) && !nodeToPodMap.get(destinationNodeName)?.includes(dstName)) {
        nodeToPodMap.get(destinationNodeName)?.push(dstName);
      } else if (!nodeToPodMap.has(destinationNodeName)) {
        nodeToPodMap.set(destinationNodeName, [dstName]);
      }
  
      // determine how much traffic is being sent
      let pod_src = sourceNodeName+'_pod_'+srcName;
      let pod_dst = destinationNodeName+'_pod_'+dstName;
      let svc_dst = 'svc_'+destinationServicePortName;
      let dests = new Map<string, number>();
      dests.set(pod_dst, octetDeltaCount);
      if (destinationServicePortName !== '') {
        dests.set(svc_dst, octetDeltaCount);
      }
      if (srcToDestMap.has(pod_src)) {
        if (srcToDestMap.get(pod_src)?.has(pod_dst)) {
          srcToDestMap.get(pod_src)?.set(pod_dst, octetDeltaCount+srcToDestMap.get(pod_src)?.get(pod_dst));
        } else {
          srcToDestMap.get(pod_src)?.set(pod_dst, octetDeltaCount);
        }
        if (destinationServicePortName === '') {
          continue;
        } else if (srcToDestMap.get(pod_src)?.has(svc_dst)) {
          srcToDestMap.get(pod_src)?.set(svc_dst, octetDeltaCount+srcToDestMap.get(pod_src)?.get(svc_dst));
        } else {
          srcToDestMap.get(pod_src)?.set(svc_dst, octetDeltaCount);
        }
      } else {
        srcToDestMap.set(pod_src, dests);
      }
    }
  
    // format pods inside node within graph string
    nodeToPodMap.forEach((pods, nodename) => {
      let str = 'subgraph ' + nodename + '\n';
      pods.forEach((pod) => {
        str += nodename + '_pod_' + pod + '(' + pod + ');\n';
      });
      str += 'end;\n';
      graphString += str;
    });
    // format arrows to services and pods within graph string
    let prefixes = ['', 'K', 'M', 'G', 'T'];
    srcToDestMap.forEach((destsToBytes, src) => {
      destsToBytes.forEach((bytes, dest) => {
        let usedpref = Math.floor(Math.log(bytes) / Math.log(1000));
        if (usedpref > 4) {usedpref = 4};
        let str = src + ' -- ' + bytes/(Math.pow(1000, usedpref)) + ' ' + prefixes[usedpref] + 'B --> ' + dest + ';\n';
        graphString += str;
      });
    });
  }

  function layerSevenGraph() {
    function getColorFromStatus(httpStatus: string) {
      // colors that correspond to each of the types of http response code; i.e. 4xx and 5xx codes return red to symbolize errors
      const colors = ['orange', 'green', 'blue', 'red', 'red'];
      let statusType = +httpStatus.charAt(0);
      if (statusType < 1 || statusType > 5) {
        // nothing else returns purple, purple indicates an error in the httpVals field
        return 'purple';
      }
      return colors[statusType-1];
    }

    let ctr = 0;
    for (let i = 0; i < frame.length; i++) {
      const sourcePodName = sourcePodNames?.values.get(i);
      const destinationPodName = destinationPodNames?.values.get(i);
      const sourceIP = sourceIPs?.values.get(i);
      const sourcePort = sourceTransportPorts?.values.get(i);
      const destinationIP = destinationIPs?.values.get(i);
      const destinationPort = destinationTransportPorts?.values.get(i);
      const httpVals = httpValsList?.values.get(i);

      let httpValsJSON = JSON.parse(httpVals);
      for (const txId in httpValsJSON) {
        let graphLine = '';
        if (sourcePodName !== '') {
          graphLine = sourcePodName;
        } else {
          graphLine = sourceIP + ':' + sourcePort;
        }
        graphLine = graphLine + ' --' + httpValsJSON[txId].length + '--> ';
        if (destinationPodName !== '') {
          graphLine = graphLine + destinationPodName;
        } else {
          graphLine = graphLine + destinationIP + ':' + destinationPort;
        }
        graphString = graphString + graphLine + '\n';
        let styleLine = 'linkStyle ' + ctr + ' stroke: ' + getColorFromStatus(httpValsJSON.status+'') + '\n';
        styleString = styleString + styleLine;
        ctr += 1;
      }
    }

    graphString = graphString + styleString;
  }

  if (options.layerFour) {
    layerFourGraph();
  } else {
    layerSevenGraph();
  }
  
  try {
    mermaid.parse(graphString);
  } catch (err) {
    console.log('incorrect graph syntax for graph:\n'+graphString);
    console.log(err);
    return (
      <div><p>Incorrect Graph Syntax</p></div>
    );
  }

  return (
    <div className='wrapper'>
      <DiagramPanelController 
        graphString={graphString}
        boxColor={boxColor}
        layerLevel={layerLevel}
        theme={theme}
      ></DiagramPanelController>
    </div>
  );
};
