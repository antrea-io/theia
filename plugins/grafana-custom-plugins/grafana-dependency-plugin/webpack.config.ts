// webpack.config.ts
import type { Configuration } from 'webpack';
import { merge } from 'webpack-merge';
import grafanaConfig from './.config/webpack/webpack.config';

const pluginJson = require('./src/plugin.json');

const config = async (env): Promise<Configuration> => {
  const baseConfig = await grafanaConfig(env);

  return merge(baseConfig, {
    // Add custom config here...
    output: {
     // -- this is the important change
      publicPath: `public/plugins/${pluginJson.id}/`,
    },
  });
};

export default config;