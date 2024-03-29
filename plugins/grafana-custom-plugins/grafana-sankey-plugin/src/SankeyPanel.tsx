import React from 'react';
import { PanelProps } from '@grafana/data';
import { SankeyOptions } from 'types';
import Chart from 'react-google-charts';

interface Props extends PanelProps<SankeyOptions> {}

export const SankeyPanel: React.FC<Props> = ({ options, data, width, height }) => {
  let result = [
    ['From', 'To', 'Bytes'],
    ['Source N/A', 'Destination N/A', 1],
  ];
  let sources = data.series
    .map((series) => series.fields.find((field) => field.name.toLowerCase() === 'source'))
    .map((field) => {
      let record = field?.values as any;
      // The variable 'record' has different types when used with Jest and Webpack.
      // In the Webpack environment, it is a 'vectorArray,' and 'record?.buffer' is an array.
      // In the Jest environment, 'record' is already an array, and 'record?.buffer' will be 'undefined',
      // which can break the test. The following 'if' condition is added to ensure
      // successful unit test execution with Jest.
      if(Array.isArray(record)) {
        return record
      } else {
        return record?.buffer;
      }
    })[0];
  if (sources !== undefined) {
    let destinations = data.series
      .map((series) => series.fields.find((field) => field.name.toLowerCase() === 'destination'))
      .map((field) => {
        let record = field?.values as any;
        if(Array.isArray(record)) {
          return record
        } else {
          return record?.buffer;
        }
      })[0];
    let destinationIPs = data.series
      .map((series) => series.fields.find((field) => field.name.toLowerCase() === 'destinationip'))
      .map((field) => {
        let record = field?.values as any;
        if(Array.isArray(record)) {
          return record
        } else {
          return record?.buffer;
        }
      })[0];
    let bytes = data.series
      .map((series) => series.fields.find((field) => field.name.toLowerCase() === 'bytes'))
      .map((field) => {
        let record = field?.values as any;
        if(Array.isArray(record)) {
          return record
        } else {
          return record?.buffer;
        }
      })[0];
    let n = sources.length;
    for (let i = 0; i < n; i++) {
      if (bytes[i] === 0) {
        continue;
      }
      let record = [];
      let source = sources[i];
      let destination = destinations[i];
      if (source === '') {
        source = 'N/A';
      }
      if (destination === '') {
        if (destinationIPs[i] === '') {
          destination = 'N/A';
        } else {
          destination = destinationIPs[i];
        }
      } else {
        // Google Chart will not render if cycle exists. Direct cycle: a->a
        // (e.g. intra-Node traffic in node_to_node_flows dashboard); Indirect
        // cycle: a->b, b->c, c->a. Add an extra space to all the destination
        // names to avoid introducing cycles.
        destination = destination + ' ';
      }
      record.push(source);
      record.push(destination);
      record.push(bytes[i]);
      if (i === 0) {
        result = [['From', 'To', 'Bytes']];
      }
      result.push(record);
    }
  }
  return (
    <div>
      <Chart width={600} height={'600px'} chartType="Sankey" loader={<div>Loading Chart</div>} data={result} />
    </div>
  );
};
