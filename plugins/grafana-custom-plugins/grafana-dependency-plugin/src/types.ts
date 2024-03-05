type BoxColor = 'red' | 'yellow' | 'green' | 'blue'

export interface DependencyOptions {
    groupByPodLabel: boolean;
    layerFour: boolean;
    labelName: string;
    color: BoxColor;
}
