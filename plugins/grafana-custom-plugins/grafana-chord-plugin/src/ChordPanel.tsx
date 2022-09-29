import React, { useRef, useEffect } from 'react';
import { DataFrame, PanelProps } from '@grafana/data';
import { withTheme2, Themeable2 } from '@grafana/ui';
import { ChordOptions } from 'types';
import * as d3 from 'my-d3';

interface Props extends Themeable2, PanelProps<ChordOptions> {}

export const UnthemedChordPanel: React.FC<Props> = ({ options, data, width, height, theme }) => {
  const d3Container = useRef(null);

  // GetFieldVal reads field's value from data series
  function GetFieldVal(fieldName: string) {
    return data.series
      .map((series: DataFrame) => series.fields.find((field: any) => field.name.toLowerCase() === fieldName.toLowerCase()))
      .map((field: any) => {
        let record = field?.values as any;
        return record?.buffer;
      })[0];
  }

  useEffect(() => {
    if (data && d3Container.current) {
      // Remove added 'g' and '.tooltip' elements before each draw to remove
      // have multiple rendered graphs while resizing
      d3.select('g').remove();
      d3.selectAll('.tooltip').remove();

      const svg = d3.select(d3Container.current);

      const margin = { left: 0, top: 0, right: 0, bottom: 0 };
      let onClick = false;
      const w = width + margin.left + margin.right;
      const h = height + margin.top + margin.bottom;
      const x = width / 2 + margin.left;
      const y = height / 2 + margin.top;

      // step-1: process data

      let records = [];
      let sourcePods = GetFieldVal('srcPod');
      if (sourcePods !== undefined) {
        let sourcePorts = GetFieldVal('srcPort');
        let destinationSvcs = GetFieldVal('dstSvc');
        let destinationSvcPorts = GetFieldVal('dstSvcPort');
        let destinationPods = GetFieldVal('dstPod');
        let destinationPorts = GetFieldVal('dstPort');
        let destinationIPs = GetFieldVal('dstIP');
        let bytes = GetFieldVal('bytes');
        let reverseBytes = GetFieldVal('revBytes');
        let egressNPs = GetFieldVal('egressNetworkPolicyName');
        let egressRuleActions = GetFieldVal('egressNetworkPolicyRuleAction');
        let ingressNPs = GetFieldVal('ingressNetworkPolicyName');
        let ingressRuleActions = GetFieldVal('ingressNetworkPolicyRuleAction');
        let n = sourcePods.length;
        for (let i = 0; i < n; i++) {
          let record = [];
          let source = sourcePods[i];
          let sourcePort = sourcePorts[i];
          let destination = destinationSvcs[i];
          let destinationPort = destinationSvcPorts[i];
          if (destination === '' || destination === null) {
            if (destinationPods[i] === '/' || destinationPods[i] === '' || destinationPods[i] === null) {
              destination = destinationIPs[i];
            } else {
              destination = destinationPods[i];
              destinationPort = destinationPorts[i];
            }
          }
          record.push(source);
          record.push(sourcePort);
          record.push(destination);
          record.push(destinationPort);
          record.push(bytes[i]);
          record.push(reverseBytes[i]);
          record.push(egressNPs[i]);
          record.push(egressRuleActions[i]);
          record.push(ingressNPs[i]);
          record.push(ingressRuleActions[i]);
          records.push(record);
        }
      }

      const names = Array.from(new Set(records.flatMap((d) => [d[0], d[2]])));
      const color = d3.scaleOrdinal(d3.schemeSet3);
      const index = new Map(names.map((name: string, i) => [name, i]));
      // Used to store byte counts of each connection
      const matrix = Array.from(index, () => new Array(names.length).fill(0));
      type Connection = {
        source: string;
        sourcePort: number;
        destination: string;
        destinationPort: number;
        egressNP: string;
        ingressNP: string;
        egressRuleAction: number;
        ingressRuleAction: number;
        bytes: number;
        reverseBytes: number;
      };
      // Used to store metadata of each connection
      const connMap: Map<string, Connection> = new Map();
      const ruleActionMap: Map<number, string> = new Map([
        [1, 'Allow'],
        [2, 'Drop'],
        [3, 'Reject'],
      ]);

      for (let record of records) {
        let source: string = record[0];
        let sourcePort: number = record[1];
        let destination: string = record[2];
        let destiantionPort: number = record[3];
        let byte: number = record[4];
        let reverseByte: number = record[5];
        let egressNP: string = record[6];
        let egressRuleAction: number = record[7];
        let ingressNP: string = record[8];
        let ingressRuleAction: number = record[9];
        // Enter connection entry into chord matrix
        matrix[index.get(source)!][index.get(destination)!] += byte;
        // Enter connection entry into connMap
        // Enter key
        let idxStr: string = [index.get(source)!, index.get(destination)!].join(',');
        // Enter value
        let conn: Connection = {
          source: source,
          sourcePort: sourcePort,
          destination: destination,
          destinationPort: destiantionPort,
          egressNP: egressNP,
          egressRuleAction: egressRuleAction,
          ingressNP: ingressNP,
          ingressRuleAction: ingressRuleAction,
          bytes: byte,
          reverseBytes: reverseByte,
        };
        connMap.set(idxStr, conn);
      }

      // step-2: draw chord diagram

      const denyColor = '#EE4B2B';
      const allowColor = '#228B22';
      const innerRadius = Math.min(w, h) * 0.5 - 100;
      const outerRadius = innerRadius + 10;
      const chord = d3
        .chordDirected()
        .padAngle(10 / innerRadius)
        .sortSubgroups(d3.descending)
        .sortChords(d3.descending);
      const arc = d3.arc<d3.ChordGroup>().innerRadius(innerRadius).outerRadius(outerRadius);
      const ribbon = d3
        .ribbonArrow<d3.Chord, d3.ChordSubgroup>()
        .radius(innerRadius - 1)
        .padAngle(1 / innerRadius);
      const tooltip = d3
        .select('body')
        .append('div')
        .attr('class', 'tooltip')
        .style('opacity', 0)
        .style('position', 'absolute')
        .style('z-index', '10')
        .style('background-color', theme.colors.background.secondary)
        .style('border', 'solid')
        .style('border-width', '2px')
        .style('border-radius', '5px')
        .style('padding', '5px');

      const diagram = svg
        .attr('width', w)
        .attr('height', h)
        .append('g')
        .attr('transform', 'translate(' + x + ',' + y + ')');

      // Create a transparent background rect as a click area
      diagram
        .append('rect')
        .attr('width', w)
        .attr('height', h)
        .attr('transform', 'translate(' + -x + ',' + -y + ')')
        .style('opacity', 0)
        .on('click', function () {
          if (onClick === true) {
            ribbons.transition().style('opacity', 0.8);
            onClick = false;
          }
        });

      // Give the data matrix to d3.chord(): it will calculates all the info we
      // need to draw arcs and ribbons
      const res = chord(matrix);

      // Add outer arcs
      const arcs = diagram
        .datum(res)
        .append('g')
        .selectAll('g')
        .data(function (d) {
          return d.groups;
        })
        .enter()
        .append('g');

      arcs
        .append('path')
        .on('mouseover', function (event, d) {
          if (onClick === true) {
            return;
          }
          let i = d.index;
          ribbons
            .filter(function (d) {
              return d.source.index !== i && d.target.index !== i;
            })
            .transition()
            .style('opacity', 0.1);
        })
        .on('mouseout', function (event, d) {
          if (onClick === true) {
            return;
          }
          let i = d.index;
          ribbons
            .filter(function (d) {
              return d.source.index !== i && d.target.index !== i;
            })
            .transition()
            .style('opacity', 0.8);
        })
        .on('click', function (event, d) {
          event.stopPropagation();
          onClick = true;
          let i = d.index;
          ribbons
            .filter(function (d) {
              return d.source.index !== i && d.target.index !== i;
            })
            .transition()
            .style('opacity', 0.1);
        })
        .style('fill', function (d) {
          return color(names[d.index]);
        })
        .style('stroke', 'black')
        .attr('id', function (d) {
          return 'group' + d.index;
        })
        .attr('d', arc);

      // Add text labels to arcs
      const labels = diagram
        .append('g')
        .selectAll('text')
        .data(res.groups)
        .enter()
        .append('text')
        .each(function (d: any) {
          d.angle = (d.startAngle + d.endAngle) / 2;
        })
        .attr('dy', '.35em')
        .attr('text-anchor', function (d: any) {
          return d.angle > Math.PI ? 'end' : null;
        })
        .attr('transform', function (d: any) {
          return (
            'rotate(' +
            ((d.angle * 180) / Math.PI - 90) +
            ')' +
            'translate(' +
            (innerRadius + 15) +
            ')' +
            (d.angle > Math.PI ? 'rotate(180)' : '')
          );
        })
        .attr('opacity', 0.9);

      // Add namespace to label
      labels
        .append('tspan')
        .style('fill', theme.colors.text.primary)
        .attr('x', 0)
        .attr('dy', 0)
        .text(function (chords, i) {
          let s = names[i].split('/');
          if (s[0] !== undefined) {
            return s[0];
          } else {
            return '';
          }
        });

      // Add Pod/Service name to label
      labels
        .append('tspan')
        .style('fill', theme.colors.text.primary)
        .attr('x', 0)
        .attr('dy', 15)
        .text(function (chords, i) {
          let s = names[i].split('/');
          if (s[1] !== undefined) {
            return s[1];
          } else {
            return '';
          }
        });

      // Add inner ribbons
      const ribbons = diagram
        .datum(res)
        .append('g')
        .selectAll('path')
        .data(function (d) {
          return d;
        })
        .enter()
        .append('path');

      ribbons
        .attr('class', 'ribbons')
        .attr('d', ribbon)
        .attr('stroke', 'black')
        .style('opacity', 0.8)
        // Set ribbon color, deny -> red, allow -> green, others -> source group color
        .style('fill', function (d) {
          const idxStr = [d.source.index, d.target.index].join(',');
          const egressRuleAction = connMap.get(idxStr)?.egressRuleAction;
          const ingressRuleAction = connMap.get(idxStr)?.ingressRuleAction;
          if (egressRuleAction === 2 || egressRuleAction === 3 || ingressRuleAction === 2 || ingressRuleAction === 3) {
            return denyColor;
          }
          if (egressRuleAction === 1 || ingressRuleAction === 1) {
            return allowColor;
          }
          return color(names[d.source.index]);
        })
        // Add tooltips to ribbons on mouseover event
        .on('mouseover', function (event, d) {
          const idxStr = [d.source.index, d.target.index].join(',');
          const conn = connMap.get(idxStr)!;
          let source = conn.source;
          if (conn.sourcePort !== 0) {
            source += `:` + conn.sourcePort;
          }
          let destination = conn.destination;
          if (conn.destinationPort !== 0) {
            destination += `:` + conn.destinationPort;
          }
          let texts =
            `
          <table style="margin-top: 2.5px;">
          <tr><td>From: </td><td style="text-align: right">` +
            source +
            `</td></tr><tr><td>To: </td><td style="text-align: right">` +
            destination;
          // Add egressNetworkPolicy metadata
          if (conn.egressNP !== '') {
            texts +=
              `</td></tr>
            <tr><td>Egress NetworkPolicy name: </td><td style="text-align: right">` +
              conn.egressNP +
              `</td></tr>
            <tr><td>Egress NetworkPolicy Rule Action: </td><td style="text-align: right">` +
              ruleActionMap.get(conn.egressRuleAction);
          }
          // Add ingressNetworkPolicy metadata
          if (conn.ingressNP !== '') {
            texts +=
              `</td></tr>
            <tr><td>Ingress NetworkPolicy name: </td><td style="text-align: right">` +
              conn.ingressNP +
              `</td></tr>
            <tr><td>Ingress NetworkPolicy Rule Action: </td><td style="text-align: right">` +
              ruleActionMap.get(conn.ingressRuleAction);
          }
          // Add bytes and reverseBytes
          texts +=
            `</td></tr>
          <tr><td>Bytes: </td><td style="text-align: right">` +
            conn.bytes +
            `</td></tr>
          <tr><td>Reverse Bytes: </td><td style="text-align: right">` +
            conn.reverseBytes +
            `</td></tr>
                  </table>
                  `;
          return tooltip
            .style('opacity', 1)
            .html(texts)
            .style('left', event.pageX + 10 + 'px')
            .style('top', event.pageY - 10 + 'px');
        })
        .on('mousemove', function (event, d) {
          return tooltip.style('top', event.pageY - 10 + 'px').style('left', event.pageX + 10 + 'px');
        })
        .on('mouseout', function () {
          return tooltip.style('opacity', 0);
        });
    }
  });

  return <svg className="d3-component" width={width} height={height} ref={d3Container} />;
};

export const ChordPanel = withTheme2(UnthemedChordPanel);
