'use strict';

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const path = require('node:path');

const ROOT = path.join(__dirname, '..', 'public');
const html = fs.readFileSync(path.join(ROOT, 'index.html'), 'utf8');
const app = fs.readFileSync(path.join(ROOT, 'app', 'app.js'), 'utf8');

test('global search button opens a functional command palette', () => {
  assert.match(html, /@click="openSearch\(\)"/);
  assert.match(html, /id="global-search-input"/);
  assert.match(html, /x-for="\(item, index\) in searchResults"/);
  assert.doesNotMatch(html, /Search is not available yet/);
});

test('Ctrl or Command K is captured before the browser shortcut', () => {
  assert.match(app, /event\.ctrlKey \|\| event\.metaKey/);
  assert.match(app, /event\.preventDefault\(\)/);
  assert.match(app, /event\.stopPropagation\(\)/);
  assert.match(app, /addEventListener\('keydown', this\._searchKeyHandler, true\)/);
  assert.match(app, /if \(this\.page !== 'app'\) return/);
  assert.doesNotMatch(app, /if \(this\.page !== 'dashboard'\) return/);
});
