import { configure, shallow } from 'enzyme';
import Adapter from 'enzyme-adapter-react-16';
import { TablePanel } from './TablePanel';
import { LoadingState, PanelProps, TimeRange, toDataFrame } from '@grafana/data';
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
    let component = shallow(<TablePanel {...props} />);
    React.useLayoutEffect = React.useEffect;
    console.log('component text: '+component.render().text());
    let renderedHtmlString = component.render().text();
    expect(renderedHtmlString.includes('10.10.1.4:42162')).toEqual(true);
    expect(renderedHtmlString.includes('93.184.216.34:80')).toEqual(true);
    expect(renderedHtmlString.includes('example.com')).toEqual(true);
    expect(renderedHtmlString.includes('/')).toEqual(true);
    expect(renderedHtmlString.includes('curl/7.88.1')).toEqual(true);
    expect(renderedHtmlString.includes('text/html')).toEqual(true);
    expect(renderedHtmlString.includes('GET')).toEqual(true);
    expect(renderedHtmlString.includes('200')).toEqual(true);
    expect(renderedHtmlString.includes('1256')).toEqual(true);
    // expect(
    //   component.contains(
    //     <td className="MuiTableCell-root MuiTableCell-body MuiTableCell-sizeMedium css-qlgmli-MuiTableCell-root">10.10.1.4:42162</td>
    //     // <MaterialReactTable 
    //     //   columns={columns} 
    //     //   data={tableData}
    //     //   enableExpanding
    //     //   enableColumnResizing
    //     //   layoutMode='grid'
    //     //   muiTableHeadCellProps={{
    //     //     sx: {
    //     //       flex: '0 0 auto',
    //     //     },
    //     //   }}
    //     //   muiTableBodyCellProps={{
    //     //     sx: {
    //     //       flex: '0 0 auto',
    //     //     },
    //     //   }}
    //     // />
    //   )
    // ).toEqual(true);
  });
});