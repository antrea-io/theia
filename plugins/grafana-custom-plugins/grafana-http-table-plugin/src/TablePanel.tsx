import React, { useMemo } from 'react';
import {config} from '@grafana/runtime'
import { PanelProps } from '@grafana/data';
import { SimpleOptions } from 'types';
import { MaterialReactTable, type MRT_ColumnDef} from 'material-react-table';
import { ThemeProvider, createTheme } from '@mui/material';

interface Props extends PanelProps<SimpleOptions> {}

export const TablePanel: React.FC<Props> = ({ options, data, width, height }) => {
  const frame = data.series[0];
  const columns = useMemo<Array<MRT_ColumnDef<FlowRow>>>(
    () => [
      {header: 'Source', accessorKey: 'source'},
      {header: 'Destination', accessorKey: 'destination'},
      {header: 'Transaction ID', accessorKey: 'txId'},
      {header: 'Hostname', accessorKey: 'hostname'},
      {header: 'URL', accessorKey: 'url'},
      {header: 'HTTP User Agent', accessorKey: 'http_user_agent'},
      {header: 'HTTP Content Type', accessorKey: 'http_content_type'},
      {header: 'HTTP Method', accessorKey: 'http_method'},
      {header: 'Protocol', accessorKey: 'protocol'},
      {header: 'Status', accessorKey: 'status'},
      {header: 'Length', accessorKey: 'length'},
    ],
    [],
  );
  const sourceIPs = frame.fields.find((field) => field.name === 'sourceIP');
  const destinationIPs = frame.fields.find((field) => field.name === 'destinationIP');
  const sourceTransportPorts = frame.fields.find((field) => field.name === 'sourceTransportPort');
  const destinationTransportPorts = frame.fields.find((field) => field.name === 'destinationTransportPort');
  const httpValsList = frame.fields.find((field) => field.name === 'httpVals');
  const defaultMaterialTheme = createTheme({palette: {
    mode: config.theme2.isDark ? 'dark': 'light',
  }});

  let tableData: FlowRow[] = [];
  interface FlowRow {
    source: string,
    destination: string,
    txId: number,
    hostname: string,
    url: string,
    http_user_agent: string,
    http_content_type: string,
    http_method: string,
    protocol: string,
    status: number,
    length: number,
    subRows: FlowRow[],
  }
  for (let i = 0; i < frame.length; i++) {
    const sourceIP = sourceIPs?.values.get(i);
    const destinationIP = destinationIPs?.values.get(i);
    const sourcePort = sourceTransportPorts?.values.get(i);
    const destinationPort = destinationTransportPorts?.values.get(i);
    const httpVals = httpValsList?.values.get(i);
    let httpValsJSON: any;
    if (httpVals !== undefined) {
      httpValsJSON = JSON.parse(httpVals);
    }
    const rowData: FlowRow = {
      source: '',
      destination: '',
      txId: 0,
      hostname: '',
      url: '',
      http_user_agent: '',
      http_content_type: '',
      http_method: '',
      protocol: '',
      status: 0,
      length: 0,
      subRows: [],
    };
    function setTableRow(source: string, destination: string, txId: number, hostname: string, url: string, http_user_agent: string, http_content_type: string, http_method: string, protocol: string, status: number, length: number) {
      if (txId === 0) {
        rowData.source = source;
        rowData.destination = destination;
        rowData.txId = txId;
        rowData.hostname = hostname;
        rowData.url = url;
        rowData.http_user_agent = http_user_agent;
        rowData.http_content_type = http_content_type;
        rowData.http_method = http_method;
        rowData.protocol = protocol;
        rowData.status = status;
        rowData.length = length;
      } else {
        const row: FlowRow = {
          source: source,
          destination: destination,
          txId: txId,
          hostname: hostname,
          url: url,
          http_user_agent: http_user_agent,
          http_content_type: http_content_type,
          http_method: http_method,
          protocol: protocol,
          status: status,
          length: length,
          subRows: [],
        }
        rowData.subRows.push(row);
      }
    }
    for (const txId in httpValsJSON) {
      setTableRow(sourceIP+':'+sourcePort, destinationIP+':'+destinationPort, +txId, httpValsJSON[txId].hostname, httpValsJSON[txId].url, httpValsJSON[txId].http_user_agent, httpValsJSON[txId].http_content_type, httpValsJSON[txId].http_method, httpValsJSON[txId].protocol, httpValsJSON[txId].status, httpValsJSON[txId].length);
      tableData.push(rowData);
    }
  }

  return (
    <div className='tableWrapper'>
      <ThemeProvider theme={defaultMaterialTheme}>
        <MaterialReactTable 
          data={tableData}
          columns={columns}
          enableExpanding
          enableColumnResizing
          layoutMode='grid'
          muiTableHeadCellProps={{
            sx: {
              flex: '0 0 auto',
            },
          }}
          muiTableBodyCellProps={{
            sx: {
              flex: '0 0 auto',
            },
          }}
        />
      </ThemeProvider>
    </div>
  );
};
