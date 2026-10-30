'use strict';

// Regression test: empty tooltip fields must be omitted instead of rendering
// the legacy "|" placeholder. Located outside public/ so it is not embedded
// in the Go binary.
//
// Run with: node --test unyport/frontend/tests

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const APP_DIR = path.join(__dirname, '..', 'public', 'app');
const HOVER_SCRIPT = path.join(APP_DIR, 'networkmap-hover.js');

global.document = { readyState: 'loading', addEventListener() {} };
global.window = {};

const { hasTooltipValue, firstText, rowsHtml, applyOptionalText } = require(HOVER_SCRIPT);

test('values without content are not considered present', () => {
  for (const value of [null, undefined, '', '   ', '\t\n', '|', ' | ', '|  ']) {
    assert.equal(hasTooltipValue(value), false, `expected ${JSON.stringify(value)} to be empty`);
  }
});

test('real values are preserved, including numeric zero', () => {
  for (const value of ['0', '0 B/s', '192.168.4.3', 'UP', 'Interface', 'br-wan']) {
    assert.equal(hasTooltipValue(value), true, `expected ${JSON.stringify(value)} to be present`);
  }
});

test('rowsHtml drops rows whose value is empty or a pipe placeholder', () => {
  const html = rowsHtml([
    { key: 'Type', value: 'Interface' },
    { key: 'IP address', value: '|' },
    { key: 'Bridge', value: '' },
    { key: 'Bridge IP', value: null },
    { key: 'VLAN', value: '   ' },
    { key: 'RX rate', value: '1.2 Mb/s' }
  ]);

  assert.equal((html.match(/nm-detail-row/g) || []).length, 2);
  assert.match(html, /Type/);
  assert.match(html, /RX rate/);
  assert.doesNotMatch(html, /IP address/);
  assert.doesNotMatch(html, /Bridge/);
  assert.doesNotMatch(html, /VLAN/);
  assert.doesNotMatch(html, />\|</);
});

test('rowsHtml trims values and keeps HTML escaped', () => {
  const html = rowsHtml([{ key: '<b>Device</b>', value: '  <script>x</script>  ' }]);
  assert.match(html, /&lt;b&gt;Device&lt;\/b&gt;/);
  assert.match(html, /&lt;script&gt;x&lt;\/script&gt;/);
  assert.doesNotMatch(html, /<script>/);
  assert.equal((html.match(/nm-detail-row/g) || []).length, 1);
});

test('rowsHtml tolerates missing row collections', () => {
  assert.equal(rowsHtml(undefined), '');
  assert.equal(rowsHtml([]), '');
  assert.equal(rowsHtml([null, { key: 'A', value: '|' }]), '');
});

test('applyOptionalText hides empty subtitles instead of printing a placeholder', () => {
  const el = { textContent: '|', hidden: false };
  applyOptionalText(el, '|');
  assert.equal(el.textContent, '');
  assert.equal(el.hidden, true);

  applyOptionalText(el, '   ');
  assert.equal(el.hidden, true);

  applyOptionalText(el, ' br-wan ');
  assert.equal(el.textContent, 'br-wan');
  assert.equal(el.hidden, false);
});

test('firstText skips placeholder candidates and keeps real ones', () => {
  assert.equal(firstText('|', '', 'Info'), 'Info');
  assert.equal(firstText(undefined, null, 'Network node'), 'Network node');
  assert.equal(firstText(' br-wan ', 'Interface'), 'br-wan');
  assert.equal(firstText('|'), '');
  assert.equal(firstText('  '), '');
});

test('the hover renderer has no pipe fallback left', () => {
  const source = fs.readFileSync(HOVER_SCRIPT, 'utf8');
  assert.doesNotMatch(source, /\|\|\s*'\|'/);
  assert.doesNotMatch(source, />\|</);
});
