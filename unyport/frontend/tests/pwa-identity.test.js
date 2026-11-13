const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const root = path.resolve(__dirname, '..', 'public');
const html = fs.readFileSync(path.join(root, 'index.html'), 'utf8');
const manifest = JSON.parse(fs.readFileSync(path.join(root, 'manifest.json'), 'utf8'));

test('public PWA identity is UnyPort only', () => {
  assert.equal(manifest.name, 'UnyPort');
  assert.equal(manifest.short_name, 'UnyPort');
  assert.match(html, /<meta name="apple-mobile-web-app-title" content="UnyPort">/);
  assert.doesNotMatch(html, /Corporate/i);
  assert.doesNotMatch(JSON.stringify(manifest), /Corporate/i);
});
