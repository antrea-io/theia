import { PanelPlugin } from '@grafana/data';
import { DependencyOptions } from './types';
import { DependencyPanel } from './DependencyPanel';

export const plugin = new PanelPlugin<DependencyOptions>(DependencyPanel).setPanelOptions((builder) => builder);
