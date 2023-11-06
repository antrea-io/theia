import React from 'react';
import mermaid from 'mermaid';

export interface DiagramPanelControllerProps {
  boxColor: any;
  layerLevel: string;
  graphString: string;
  theme: any;
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
      console.log('binding diagram functions');
      this.bindFunctions = bindFunctions;
    }
  }

  componentDidMount() {
    this.initializeMermaid();
  }

  componentDidUpdate(prevProps: DiagramPanelControllerProps) {
    if (prevProps.graphString !== this.props.graphString ||
      prevProps.boxColor !== this.props.boxColor ||
      prevProps.layerLevel !== this.props.layerLevel) {
      this.initializeMermaid();
    }
  }

  initializeMermaid() {
    mermaid.initialize({
      startOnLoad: true,
      theme: 'base',
      themeVariables: {
        primaryColor: this.props.boxColor,
        secondaryColor: this.props.theme.colors.background.canvas,
        tertiaryColor: this.props.theme.colors.background.canvas,
        primaryTextColor: this.props.theme.colors.text.maxContrast,
        lineColor: this.props.theme.colors.text.maxContrast,
      },
    });
    if (this.diagramRef) {
      const diagramId = 'graphDiv'+this.props.layerLevel;
      mermaid.render(diagramId, this.props.graphString, (svg, bindFunctions) => {
        this.diagramRef.innerHTML = svg;
        this.bindFunctions?.(this.diagramRef);
      });
    }
  }

  render() {
    if (this.props.layerLevel === 'four') {
      return (
        <div className='diagram-wrapper'>
          <div 
            ref={this.setDiagramRef}
            className={`diagram-${this.props.layerLevel}`}
          ></div>
          <div><p>In this graph, Pods are grouped by parent Node, and arrows between Pods describe the quantity of data sent from source to destination in bytes.</p></div>
        </div>
      )
    } else {
      return (
        <div className='diagram-wrapper'>
          <div 
            ref={this.setDiagramRef}
            className={`diagram-${this.props.layerLevel}`}
          ></div>
          <div><p>In this graph, each flow between a source and destination is represented by an arrow labelled with the http content length of the data being sent.
            The color of the line is used to denote the status of the flow, where green represents a successful send, blue represents an informational response, yellow represents a redirectional response, and red represents an error response.
          </p></div>
        </div>
      )
    }
  }
}
