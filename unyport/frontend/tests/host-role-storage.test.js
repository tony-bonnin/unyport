const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const root = path.resolve(__dirname, '..', 'public', 'app');
const app = fs.readFileSync(path.join(root, 'app.js'), 'utf8');
const launch = fs.readFileSync(path.join(root, 'launch-screen.js'), 'utf8');

test('host role is never persisted in browser storage', () => {
  assert.doesNotMatch(app, /localStorage\.setItem\(['"]unyport-host-role/);
  assert.doesNotMatch(launch, /localStorage\.getItem\(['"]unyport-host-role/);
  assert.match(launch, /localStorage\.removeItem\(['"]unyport-host-role/);
});

