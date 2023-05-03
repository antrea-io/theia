import { PanelPlugin } from '@grafana/data';
import { DependencyOptions } from './types';
import { DependencyPanel } from './DependencyPanel';

export const plugin = new PanelPlugin<DependencyOptions>(DependencyPanel).setPanelOptions((builder) => builder.addBooleanSwitch({
    path: 'groupByPodLabel',
    name: 'Group by Pod Label',
    defaultValue: false,
}).addTextInput({
    path: 'labelName',
    name: 'Label Name',
    settings: {
        placeholder: 'app',
    },
}).addRadio({
    path: 'color',
    name: 'Box Color',
    defaultValue: 'yellow',
    settings: {
        options: [
            {
                value: 'red',
                label: 'Red',
            },
            {
                value: 'yellow',
                label: 'Yellow',
            },
            {
                value: 'green',
                label: 'Green',
            },
            {
                value: 'blue',
                label: 'Blue',
            }
        ]
    },
}));
