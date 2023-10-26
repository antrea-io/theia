// force timezone to UTC to allow tests to work regardless of local timezone
// generally used by snapshots, but can affect specific tests
process.env.TZ = 'UTC';
const esModules = ['d3', 'd3-array', 'my-d3','@grafana/ui/node_modules/ol', 'internmap', 'delaunator', 'robust-predicates'].join('|');

module.exports = {
  // Jest configuration provided by Grafana scaffolding
  ...require('./.config/jest.config'),
  transformIgnorePatterns: [`<rootDir>/node_modules/(?!${esModules})`],
};
