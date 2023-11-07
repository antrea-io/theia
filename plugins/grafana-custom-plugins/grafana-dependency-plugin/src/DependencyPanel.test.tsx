import { configure, shallow } from 'enzyme';
import Adapter from 'enzyme-adapter-react-16';
import { DependencyPanel } from './DependencyPanel';
import { DependencyOptions } from 'types';
import { LoadingState, PanelProps, TimeRange, toDataFrame } from '@grafana/data';
import React from 'react';

configure({ adapter: new Adapter() });

describe('Dependency Diagram test', () => {
  it('Should render Diagram', () => {
    let props = {} as PanelProps;
    let timeRange = {} as TimeRange;
    props.data = {
      series: [
        toDataFrame({
          refId: 'A',
          fields: [
            { name: 'sourcePodName', values: ['web-client3-1'] },
            { name: 'sourcePodLabels', values: [''] },
            { name: 'sourcePodNamespace', values: ['antrea-test-3'] },
            { name: 'sourceNodeName', values: ['kind-worker'] },
            { name: 'destinationPodName', values: ['web-server-3-86c5d4c9fc-fxqk7'] },
            { name: 'destinationPodLabels', values: [''] },
            { name: 'destinationNodeName', values: ['kind-worker'] },
            { name: 'destinationServicePortName', values: ['antrea-test-3/iperf3-3:tcp'] },
            { name: 'octetDeltaCount', values: ['162891768'] },
          ],
        }),
      ],
      state: LoadingState.Done,
      timeRange: timeRange,
    };
    props.width = 600,
    props.height = 600;
    props.options = {groupByPodLabel: false, layerFour: true, labelName: '', color: 'red'} as DependencyOptions;
    let expectedGraphString = 'graph LR;\nsubgraph kind-worker\nkind-worker_pod_web-client3-1(web-client3-1);\nkind-worker_pod_web-server-3-86c5d4c9fc-fxqk7(web-server-3-86c5d4c9fc-fxqk7);\nend;\nkind-worker_pod_web-client3-1 -- 162.891768 MB --> kind-worker_pod_web-server-3-86c5d4c9fc-fxqk7;\nkind-worker_pod_web-client3-1 -- 162.891768 MB --> svc_antrea-test-3/iperf3-3:tcp;';
    let component = shallow(<DependencyPanel {...props} />);
    let renderedHtmlString = component.render().text();
    expect(
      renderedHtmlString.includes(expectedGraphString)
    ).toEqual(true);
  });
});