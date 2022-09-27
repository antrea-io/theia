import { configure, shallow } from 'enzyme';
import Adapter from 'enzyme-adapter-react-16';
import { SankeyPanel } from './SankeyPanel';
import { LoadingState, PanelProps, TimeRange, toDataFrame } from '@grafana/data';
import React from 'react';
import { Chart } from 'react-google-charts';

configure({ adapter: new Adapter() });

describe('Sankey Diagram test', () => {
  it('Should render Chart', () => {
    let props = {} as PanelProps;
    let timeRange = {} as TimeRange;
    props.data = {
      series: [
        toDataFrame({
          refId: 'A',
          fields: [
            { name: 'source', values: ['alpine0'] },
            { name: 'destination', values: ['alpine1'] },
            { name: 'destinationIP', values: ['10.10.1.55'] },
            { name: 'bytes', values: [10000] },
          ],
        }),
      ],
      state: LoadingState.Done,
      timeRange: timeRange,
    };
    props.width = 600;
    props.height = 600;
    props.options = {};
    let data = [
      ['From', 'To', 'Bytes'],
      ['alpine0', 'alpine1 ', 10000],
    ];
    let component = shallow(<SankeyPanel {...props} />);
    expect(
      component.contains(
        <Chart chartType="Sankey" width={600} height={'600px'} loader={<div>Loading Chart</div>} data={data} />
      )
    ).toEqual(true);
  });
});
