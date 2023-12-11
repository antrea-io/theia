import { configure } from 'enzyme';
import Adapter from 'enzyme-adapter-react-16';
import { TablePanel } from './TablePanel';
import { LoadingState, PanelProps, TimeRange, toDataFrame } from '@grafana/data';
import { render, screen } from '@testing-library/react';
import '@testing-library/jest-dom';
import React from 'react';

configure({ adapter: new Adapter() });

describe('Table Plugin Test', () => {
  it('Should render Table', () => {
    let props = {} as PanelProps;
    let timeRange = {} as TimeRange;
    props.data = {
      series: [
        toDataFrame({
          refId: 'A',
          fields: [
            { name: 'sourceIP', values: ['10.10.1.4'] },
            { name: 'sourceTransportPort', values: ['42162'] },
            { name: 'destinationIP', values: ['93.184.216.34'] },
            { name: 'destinationTransportPort', values: ['80'] },
            { name: 'httpVals', values: ['{"0":{"hostname":"example.com","url":"/","http_user_agent":"curl/7.88.1","http_content_type":"text/html","http_method":"GET","protocol":"HTTP/1.1","status":200,"length":1256},"1":{"hostname":"example.com","url":"/","http_user_agent":"curl/7.88.1","http_content_type":"text/html","http_method":"GET","protocol":"HTTP/1.1","status":200,"length":1256},"2":{"hostname":"example.com","url":"/","http_user_agent":"curl/7.88.1","http_content_type":"text/html","http_method":"GET","protocol":"HTTP/1.1","status":200,"length":1256}}'] },
          ],
        }),
      ],
      state: LoadingState.Done,
      timeRange: timeRange,
    };
    props.width = 600,
    props.height = 600;
    props.options = {};
    let { unmount } = render(<TablePanel {...props} />);
    React.useLayoutEffect = React.useEffect;

    // check each column of the row is in the document and exists three times
    expect(screen.getAllByText('10.10.1.4:42162').length).toEqual(3);
    expect(screen.getAllByText('10.10.1.4:42162')[0]).toBeInTheDocument();
    expect(screen.getAllByText('93.184.216.34:80').length).toEqual(3);
    expect(screen.getAllByText('93.184.216.34:80')[0]).toBeInTheDocument();
    expect(screen.getAllByText('example.com').length).toEqual(3);
    expect(screen.getAllByText('example.com')[0]).toBeInTheDocument();
    expect(screen.getAllByText('/').length).toEqual(3);
    expect(screen.getAllByText('/')[0]).toBeInTheDocument();
    expect(screen.getAllByText('curl/7.88.1').length).toEqual(3);
    expect(screen.getAllByText('curl/7.88.1')[0]).toBeInTheDocument();
    expect(screen.getAllByText('text/html').length).toEqual(3);
    expect(screen.getAllByText('text/html')[0]).toBeInTheDocument();
    expect(screen.getAllByText('GET').length).toEqual(3);
    expect(screen.getAllByText('GET')[0]).toBeInTheDocument();
    expect(screen.getAllByText('200').length).toEqual(3);
    expect(screen.getAllByText('200')[0]).toBeInTheDocument();
    expect(screen.getAllByText('1256').length).toEqual(3);
    expect(screen.getAllByText('1256')[0]).toBeInTheDocument();

    unmount();
  });
});
