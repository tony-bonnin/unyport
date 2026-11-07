'use strict';

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const ROOT = path.join(__dirname, '..', 'public');
const sandbox = { window: { UnyPortExtendedDeviceTypes: [] }, document: {} };
vm.runInNewContext(fs.readFileSync(path.join(ROOT, 'app', 'device-icons.js'), 'utf8'), sandbox);

test('OS icon resolver recognizes explicit hostnames without replacing unrelated devices', () => {
  const icons = sandbox.window.UnyPortDeviceIcons;
  assert.equal(icons.resolveOS('WINDOWS'), 'os_windows');
  assert.equal(icons.resolveOS('alpine-xen'), 'os_alpine_linux');
  assert.equal(icons.resolveOS('ubuntu-workstation'), 'os_ubuntu');
  assert.equal(icons.resolveOS('printer-office'), '');
});

test('every catalogued OS icon is bundled', () => {
  const catalog = sandbox.window.UnyPortDeviceIcons.catalog;
  Object.keys(catalog).filter((key) => key.startsWith('os_')).forEach((key) => {
    assert.ok(fs.existsSync(path.join(ROOT, 'media', 'icons', catalog[key].asset)), catalog[key].asset);
  });
});
