import { UnthemedChordPanel } from './ChordPanel';
import { configure } from 'enzyme';
import Adapter from 'enzyme-adapter-react-16';
import React from 'react';
import { render, screen } from '@testing-library/react';
import '@testing-library/jest-dom/extend-expect';
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
            { name: 'srcPod', values: ["ns1/alpine0"] },
            { name: 'srcPort', values: [1111] }, 
            { name: 'dstSvc', values: [""] },
            { name: 'dstSvcPort', values: [0] },
            { name: 'dstPod', values: ["ns2/alpine1"] },
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
        const { unmount } = render(<UnthemedChordPanel {...props} />);
        // If useEffect is used, the component should mount and unmount successfully.
        // If it doesn't use useEffect, it will throw an error during rendering.

        // Check if the rendered component contains specific data values
        const srcPodNameElement = screen.getByText('alpine0');
        const srcPodNameSpaceElement = screen.getByText('ns1');
        const dstPodNameElement = screen.getByText('alpine1');
        const dstPodNameSpaceElement = screen.getByText('ns2');

        expect(srcPodNameElement).toBeInTheDocument();
        expect(srcPodNameSpaceElement).toBeInTheDocument();
        expect(dstPodNameElement).toBeInTheDocument();
        expect(dstPodNameSpaceElement).toBeInTheDocument();

        // Check if the color of flow is correct for denied connection
        const pathElement = document.querySelector('path[style="opacity: 0.8; fill: #EE4B2B;"]');
        expect(pathElement).toBeInTheDocument();

        unmount(); // Unmount the component to clean up.
    });
});

