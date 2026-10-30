'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const CSS_DIR = path.join(__dirname, '..', 'public', 'css');

test('hidden state overrides component display rules under strict CSP', () => {
  const main = fs.readFileSync(path.join(CSS_DIR, 'main.css'), 'utf8');
  assert.match(main, /\[hidden\]\s*\{\s*display:\s*none\s*!important;/);
});

test('mobile visibility no longer depends on inline style attributes', () => {
  const mobile = fs.readFileSync(path.join(CSS_DIR, 'mobile.css'), 'utf8');
  assert.doesNotMatch(mobile, /\[style(?:\^|\$|\*|~|\|)?=/);
  assert.match(mobile, /\.mobile-menu:not\(\[hidden\]\)/);
});
