// frontend/rollup.config.js

import svelte from 'rollup-plugin-svelte';
import terser from '@rollup/plugin-terser';
import resolve from '@rollup/plugin-node-resolve';
import css from 'rollup-plugin-css-only';
import { sentryRollupPlugin } from "@sentry/rollup-plugin";
import commonjs from '@rollup/plugin-commonjs';

import pkg from './package.json' assert {type: "json"};

const name = pkg.name.replace(/^.*\//, '');

export default {
  input: 'src/main.js',
  output: {
    sourcemap: true,
    format: 'iife',
    name: name,
    file: 'public/build/bundle.js'
  },
  plugins: [
    svelte({
      include: 'src/**/*.svelte',
      // other options like preprocess etc.
    }),
    // Extract CSS into a separate file
    css({ output: 'bundle.css' }),

    // Resolve bare module specifiers to relative paths
    resolve({
      browser: true,
      dedupe: ['svelte']
    }),
    // Conditionally apply the terser plugin for production builds
    terser(),

    commonjs({
      include: 'node_modules/**',
    }),

    sentryRollupPlugin({
      authToken: process.env.SENTRY_AUTH_TOKEN,
      org: "trading-cage",
      project: "javascript-svelte",
    }),
  ]
};
