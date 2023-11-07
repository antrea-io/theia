// force timezone to UTC to allow tests to work regardless of local timezone
// generally used by snapshots, but can affect specific tests

process.env.TZ = 'UTC';
const esModules = ['mermaid', 'd3-interpolate', '@grafana/ui/node_modules/ol', 'd3-color'].join('|');
module.exports = {
  // Jest configuration provided by Grafana scaffolding
  ...require('./.config/jest.config'),
  transformIgnorePatterns: [`<rootDir>/node_modules/(?!${esModules})`],
  setupFilesAfterEnv: [`<rootDir>/.config/jest-setup.js`],
};
