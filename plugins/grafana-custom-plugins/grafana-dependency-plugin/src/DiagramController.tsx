import React from 'react';
import mermaid from 'mermaid';

export interface DiagramPanelControllerProps {
  boxColor: any;
  layerLevelFour: boolean;
  graphString: string;
  themeColors: Map<string, string>;
}

interface DiagramPanelControllerState {
  diagramContainer?: string;
  wrapper?: string;
  legendContainer?: string;
}

export class DiagramPanelController extends React.Component<DiagramPanelControllerProps, DiagramPanelControllerState> {
  diagramRef!: HTMLDivElement;
  bindFunctions?: Function;

  constructor(props: DiagramPanelControllerProps) {
    super(props);
    this.setDiagramRef = this.setDiagramRef.bind(this);
    this.renderCallback = this.renderCallback.bind(this);
  }

  setDiagramRef(element: HTMLDivElement) {
    this.diagramRef = element;
  }

  renderCallback(svgCode: string, bindFunctions: any) {
    if (this && bindFunctions) {
      this.bindFunctions = bindFunctions;
    }
  }

  componentDidMount() {
    this.initializeMermaid();
  }

  componentDidUpdate(prevProps: DiagramPanelControllerProps) {
    if (prevProps.graphString !== this.props.graphString ||
      prevProps.boxColor !== this.props.boxColor ||
      prevProps.layerLevelFour !== this.props.layerLevelFour) {
      this.initializeMermaid();
    }
  }

  initializeMermaid() {
    mermaid.initialize({
      startOnLoad: true,
      theme: 'base',
      themeVariables: {
        primaryColor: this.props.boxColor,
        secondaryColor: this.props.themeColors['secondaryAndTertiary'],
        tertiaryColor: this.props.themeColors['secondaryAndTertiary'],
        primaryTextColor: this.props.themeColors['textAndLine'],
        lineColor: this.props.themeColors['textAndLine'],
      },
    });
    if (this.diagramRef) {
      const diagramId = this.props.layerLevelFour ? 'graphDivFour' : 'graphDivSeven';
      mermaid.render(diagramId, this.props.graphString, (svg, bindFunctions) => {
        this.diagramRef.innerHTML = svg;
        this.bindFunctions?.(this.diagramRef);
      });
    }
  }

  render() {
    if (this.props.layerLevelFour) {
      return (
        <div className='diagram-wrapper'>
          <div 
            ref={this.setDiagramRef}
            className={`diagram-four`}
          ></div>
          <div><p>In this graph, Pods are grouped by parent Node, and arrows between Pods describe the quantity of data sent from source to destination in bytes.</p></div>
          <div hidden><p>{this.props.graphString}</p></div>
        </div>
      )
    } else {
      return (
        <div className='diagram-wrapper'>
          <div 
            ref={this.setDiagramRef}
            className={`diagram-seven`}
          ></div>
          <div><p>In this graph, each flow between a source and destination is represented by an arrow labelled with the http content length of the data being sent.
            The color of the line is used to denote the status of the flow, where green represents a successful send (2xx code), blue represents an informational response (3xx code),
            yellow represents a redirectional response (1xx code), and red represents an error response (4xx or 5xx code).
          </p></div>
          <div hidden><p>{this.props.graphString}</p></div>
        </div>
      )
    }
  }
}
