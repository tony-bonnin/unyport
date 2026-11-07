'use strict';

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const path = require('node:path');

const ROOT = path.join(__dirname, '..', 'public');
const html = fs.readFileSync(path.join(ROOT, 'index.html'), 'utf8');
const css = fs.readFileSync(path.join(ROOT, 'css', 'ui.css'), 'utf8');
const charts = fs.readFileSync(path.join(ROOT, 'app', 'security-charts.js'), 'utf8');

test('security dashboard exposes its three visual summaries', () => {
  assert.match(html, /id="security-posture-chart"/);
  assert.match(html, /id="security-category-chart"/);
  assert.match(html, /id="security-service-chart"/);
  assert.match(charts, /type: 'doughnut'/);
  assert.match(charts, /type: 'bar'/);
});

test('security dashboard has responsive compact layouts', () => {
  assert.match(css, /\.security-overview\{display:grid/);
  assert.match(css, /\.security-check-list\{display:grid/);
  assert.match(css, /@media\(max-width:820px\)/);
  assert.match(css, /\.security-service-table\{overflow-x:auto\}/);
});
