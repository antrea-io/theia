import { configure, shallow } from 'enzyme';
import Adapter from 'enzyme-adapter-react-16';
import { DependencyPanel } from './DependencyPanel';
import { LoadingState, PanelProps, TimeRange, toDataFrame } from '@grafana/data';
import React from 'react';
import { DiagramPanelController } from 'DiagramController';
import { useTheme2 } from '@grafana/ui';

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
            { name: '', values: [''] },
            { name: '', values: [''] },
            { name: '', values: [''] },
            { name: '', values: [''] },
            { name: '', values: [''] },
            { name: '', values: [''] },
            { name: '', values: [''] },
            { name: '', values: [''] },
          ],
        }),
      ],
      state: LoadingState.Done,
      timeRange: timeRange,
    };
    props.width = 600,
    props.height = 600;
    props.options = {};
    let component = shallow(<DependencyPanel {...props} />);
    let theme = useTheme2();
    expect(
      component.contains(
        <DiagramPanelController boxColor={'red'} layerLevel='four' graphString='graph LR;\n' theme={theme} ></DiagramPanelController>
      )
    ).toEqual(true);
  });
});