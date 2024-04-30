#!/usr/bin/env bash

npm run build
cp public/build/bundle.js ../static/bundles/
cp public/build/bundle.css ../static/bundles/