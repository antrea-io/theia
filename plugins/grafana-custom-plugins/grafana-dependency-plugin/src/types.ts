type BoxColor = 'red' | 'yellow' | 'green' | 'blue'

export interface DependencyOptions {
    groupByPodLabel: boolean;
    labelName: string;
    color: BoxColor;
}
