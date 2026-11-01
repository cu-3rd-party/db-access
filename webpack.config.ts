import {fileURLToPath} from 'node:url';
import MiniCssExtractPlugin from 'mini-css-extract-plugin';
import type {Configuration} from 'webpack';

const isProduction = process.env.NODE_ENV !== 'development';

export default {
  mode: isProduction ? 'production' : 'development',
  entry: {
    index: fileURLToPath(new URL('web_src/js/index.ts', import.meta.url)),
  },
  devtool: isProduction ? false : 'eval-source-map',
  output: {
    path: fileURLToPath(new URL('public/assets', import.meta.url)),
    filename: 'js/[name].js',
    clean: true,
  },
  module: {
    rules: [
      {
        test: /\.ts$/i,
        exclude: /node_modules/,
        use: [{loader: 'esbuild-loader', options: {loader: 'ts', target: 'es2022'}}],
      },
      {
        test: /\.css$/i,
        use: [
          MiniCssExtractPlugin.loader,
          {loader: 'css-loader', options: {sourceMap: !isProduction, url: false}},
          {loader: 'postcss-loader', options: {postcssOptions: {plugins: ['tailwindcss']}}},
        ],
      },
    ],
  },
  plugins: [new MiniCssExtractPlugin({filename: 'css/[name].css'})],
  performance: {hints: false},
  watchOptions: {ignored: ['**/node_modules/**']},
  stats: {assetsSort: 'name', assetsSpace: Infinity, cached: false, cachedModules: false, children: false, modules: false},
} satisfies Configuration;
