'use strict';

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const path = require('node:path');

const ROOT = path.join(__dirname, '..', 'public');
const script = fs.readFileSync(path.join(ROOT, 'app', 'networkmap.js'), 'utf8');
const css = fs.readFileSync(path.join(ROOT, 'css', 'networkmap.css'), 'utf8');

test('network topology recomputes its columns when its container is resized', () => {
  assert.match(script, /new ResizeObserver/);
  assert.match(script, /nextCols === el\._nmCols/);
  assert.match(script, /renderNetworkMap\(args\[0\], args\[1\], args\[2\], args\[3\], args\[4\], el\.id\)/);
  assert.match(script, /el\._nmCols = COLS/);
});

test('network topology can shrink inside the page without widening it', () => {
  assert.match(css, /\.netmap-card\{min-width:0;max-width:100%;\}/);
  assert.match(css, /\.netmap-container\{[^}]*min-width:0;[^}]*max-width:100%/);
});
