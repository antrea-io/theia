import React from 'react';
import { PanelProps } from '@grafana/data';
import { DependencyOptions } from 'types';
import mermaid from 'mermaid';

interface Props extends PanelProps<DependencyOptions> {}

mermaid.initialize({
  startOnLoad: true
});

class Mermaid extends React.Component<any> {
  componentDidMount() {
    mermaid.contentLoaded();
  }
  render() {
    return <div className="mermaid">{this.props.chart}</div>
  }
}

export const DependencyPanel: React.FC<Props> = ({ options, data, width, height }) => {
  const frame = data.series[0];
  const sourcePodNames = frame.fields.find((field) => field.name === 'sourcePodName');
  const sourcePodLabels = frame.fields.find((field) => field.name === 'sourcePodLabels');
  const sourceNodeNames = frame.fields.find((field) => field.name === 'sourceNodeName');
  const destinationPodNames = frame.fields.find((field) => field.name === 'destinationPodName');
  const destinationPodLabels = frame.fields.find((field) => field.name === 'destinationPodLabels');
  const destinationNodeNames = frame.fields.find((field) => field.name === 'destinationNodeName');
  const destinationServicePortNames = frame.fields.find((field) => field.name === 'destinationServicePortName');
  const octetDeltaCounts = frame.fields.find((field) => field.name === 'octetDeltaCount');
  
  let nodeToPodMap = new Map<string, String[]>();
  let srcToDestMap = new Map<string, Map<string, number>>();

  let graphString = 'graph LR;\n';

  for (let i = 0; i < frame.length; i++) {
    const sourcePodName = sourcePodNames?.values.get(i);
    const sourcePodLabel = sourcePodLabels?.values.get(i);
    const sourceNodeName = sourceNodeNames?.values.get(i);
    const destinationPodName = destinationPodNames?.values.get(i);
    const destinationPodLabel = destinationPodLabels?.values.get(i);
    const destinationNodeName = destinationNodeNames?.values.get(i);
    const destinationServicePortName = destinationServicePortNames?.values.get(i);
    const octetDeltaCount = octetDeltaCounts?.values.get(i);

    let srcName = '', dstName = '';
    if (sourcePodLabel === '') {
      srcName = sourcePodName;
    } else {
      srcName = sourcePodLabel;
    }
    if (destinationPodLabel === '') {
      dstName = destinationPodName;
    } else {
      dstName = destinationPodLabel;
    }

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
    let dests = new Map<string, number>([ [pod_dst, octetDeltaCount], [svc_dst, octetDeltaCount] ]);
    if (srcToDestMap.has(pod_src)) {
      if (srcToDestMap.get(pod_src)?.has(pod_dst)) {
        srcToDestMap.get(pod_src)?.set(pod_dst, octetDeltaCount+srcToDestMap.get(pod_src)?.get(pod_dst));
      } else if(!srcToDestMap.get(pod_src)?.has(pod_dst)) {
        srcToDestMap.get(pod_src)?.set(pod_dst, octetDeltaCount);
      }
      if (srcToDestMap.get(pod_src)?.has(svc_dst)) {
        srcToDestMap.get(pod_src)?.set(svc_dst, octetDeltaCount+srcToDestMap.get(pod_src)?.get(svc_dst));
      } else if(!srcToDestMap.get(pod_src)?.has(svc_dst)) {
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

  // checking if graph syntax is valid
  mermaid.parseError = function() {
    console.log('incorrect graph syntax for graph:\n'+graphString);
    return (
      <div><p>Incorrect Graph Syntax</p></div>
    );
  }
  if (mermaid.parse(graphString)) {
    let graphElement = document.getElementsByClassName("graphDiv")[0];
    // null check because the div does not exist at this point during the first run
    if (graphElement != null) {
      let insertSvg = function(svgCode: string){
        graphElement!.innerHTML = svgCode;
      }
      mermaid.mermaidAPI.render('graphDiv', graphString, insertSvg);
    }
  }
  
  // manually display first time, since render has no target yet
  return (
    <div className="graphDiv">
      <Mermaid chart={graphString}/>
    </div>
  );
};
