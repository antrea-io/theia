import { UnthemedChordPanel } from './ChordPanel';
import { configure, mount } from 'enzyme';
import Adapter from 'enzyme-adapter-react-16';
import React from 'react';
import * as d3 from 'd3';
import { LoadingState, PanelProps, TimeRange, createTheme, toDataFrame, PanelData } from '@grafana/data'
import { Themeable2 } from '@grafana/ui';
import { ChordOptions } from 'types';

configure({ adapter: new Adapter() });

describe('Chord Panel Test', () => {
    interface Props extends Themeable2, PanelProps<ChordOptions> {}
    let props = {} as Props;
    let timeRange = {} as TimeRange;
    const mockedData: PanelData = {
        series: [
          toDataFrame({ refId: 'A', fields: [
            { name: 'srcPod', values: ["alpine0"] }, 
            { name: 'srcPort', values: [1111] }, 
            { name: 'dstSvc', values: [""] }, 
            { name: 'dstSvcPort', values: [0] },
            { name: 'dstPod', values: ["alpine1"] },
            { name: 'dstPort', values: [1112] },
            { name: 'dstIP', values: ["10.10.1.55"] },
            { name: 'bytes', values: [10000] },
            { name: 'revBytes', values: [10] },
            { name: 'egressNetworkPolicyName', values: ["egress-allow"] },
            { name: 'egressNetworkPolicyRuleAction', values: [1] },
            { name: 'ingressNetworkPolicyName', values: ["ingress-deny"] },
            { name: 'ingressNetworkPolicyRuleAction', values: [2] }
          ] }),
        ],
        state: LoadingState.Done,
        timeRange: timeRange,
      };
    props.data = mockedData;
    props.width = 600;
    props.height = 600;
    props.options = {};
    props.theme = createTheme();

    it('Should render chord', () => {
        const arcSpy = jest.spyOn(d3, 'arc');
        const ribbonSpy = jest.spyOn(d3, 'ribbonArrow');
        const scaleOrdinalSpy = jest.spyOn(d3, 'scaleOrdinal');
        const wrapper = mount(<UnthemedChordPanel {...props} />);

        expect(wrapper.find(UnthemedChordPanel).exists()).toEqual(true);
        expect(arcSpy).toHaveBeenCalled();
        expect(ribbonSpy).toHaveBeenCalled();
        expect(scaleOrdinalSpy).toHaveBeenCalled();
    });
});
