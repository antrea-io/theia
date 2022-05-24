import { PanelPlugin } from '@grafana/data';
import { ChordOptions } from './types';
import { ChordPanel } from './ChordPanel';

export const plugin = new PanelPlugin<ChordOptions>(ChordPanel).setPanelOptions((builder) => builder);
