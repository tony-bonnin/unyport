'use strict';

// Regression test: the storage section must show one card per physical disk,
// with the partitions attached as tree branches, and must not emit the legacy
// "|" placeholder in tooltips.

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const path = require('node:path');

const diskGroups = require('../public/app/disk-groups.js');

const ROOT = path.join(__dirname, '..', 'public');
const INDEX = path.join(ROOT, 'index.html');

function disk(over) {
  return Object.assign({
    device: '/dev/sda1',
    disk: '/dev/sda',
    mount: '/',
    fstype: 'ext4',
    model: 'Samsung SSD 870',
    disk_type: 'ssd',
    transport: 'sata',
    total_mb: 1000,
    used_mb: 250,
    free_mb: 750,
    used_pct: 25,
    bar_cls: '',
  }, over);
}

test('three partitions of one disk produce a single disk card', () => {
  const groups = diskGroups.group([
    disk({ device: '/dev/sda1', mount: '/' }),
    disk({ device: '/dev/sda2', mount: '/var', total_mb: 2000, used_mb: 500, free_mb: 1500 }),
    disk({ device: '/dev/sda3', mount: '/home', total_mb: 3000, used_mb: 250, free_mb: 2750 }),
  ]);

  assert.strictEqual(groups.length, 1);
  assert.strictEqual(groups[0].device, '/dev/sda');
  assert.strictEqual(groups[0].partitions.length, 3);
  assert.strictEqual(groups[0].total_mb, 6000);
  assert.strictEqual(groups[0].used_mb, 1000);
  assert.strictEqual(groups[0].used_pct, 17);
  assert.strictEqual(groups[0].volume_label, '3 volumes');
  assert.strictEqual(groups[0].model, 'Samsung SSD 870');
  assert.strictEqual(groups[0].disk_type, 'ssd');
});

test('distinct physical disks stay distinct and keep payload order', () => {
  const groups = diskGroups.group([
    disk({ device: '/dev/nvme0n1p1', disk: '/dev/nvme0n1' }),
    disk({ device: '/dev/sda1', disk: '/dev/sda' }),
    disk({ device: '/dev/nvme0n1p2', disk: '/dev/nvme0n1' }),
  ]);

  assert.deepStrictEqual(groups.map((g) => g.device), ['/dev/nvme0n1', '/dev/sda']);
  assert.strictEqual(groups[0].partitions.length, 2);
  assert.strictEqual(groups[1].volume_label, '1 volume');
});

test('missing parent disk field falls back to the device name', () => {
  assert.strictEqual(diskGroups.parentDevice('/dev/nvme0n1p3'), 'nvme0n1');
  assert.strictEqual(diskGroups.parentDevice('/dev/mmcblk0p2'), 'mmcblk0');
  assert.strictEqual(diskGroups.parentDevice('/dev/sdb12'), 'sdb');
  assert.strictEqual(diskGroups.parentDevice(''), '');

  const groups = diskGroups.group([
    { device: '/dev/vda1', total_mb: 100, used_mb: 40, free_mb: 60, used_pct: 40 },
    { device: '/dev/vda2', total_mb: 100, used_mb: 10, free_mb: 90, used_pct: 10 },
  ]);
  assert.strictEqual(groups.length, 1);
  assert.strictEqual(groups[0].device, 'vda');
  assert.strictEqual(groups[0].partitions.length, 2);
});

test('grouping tolerates empty or malformed payloads', () => {
  assert.deepStrictEqual(diskGroups.group(undefined), []);
  assert.deepStrictEqual(diskGroups.group([null, 42, {}]), []);
});

test('disk tooltip omits empty fields instead of the "|" placeholder', () => {
  const fmt = (mb) => mb + ' MB';

  const full = diskGroups.tooltip(disk({}), fmt);
  assert.match(full, /^\/dev\/sda1 · \/ · ext4 · 250 MB \/ 1000 MB \(25%\) · 750 MB free · Samsung SSD 870$/);

  const sparse = diskGroups.tooltip({ device: '/dev/sdb1', total_mb: 0, free_mb: 0 }, fmt);
  assert.strictEqual(sparse, '/dev/sdb1');
  assert.doesNotMatch(sparse, /\|/);
  assert.doesNotMatch(diskGroups.tooltip(null, fmt), /\|/);
});

test('index.html renders one card per disk with partition tree branches', () => {
  const html = fs.readFileSync(INDEX, 'utf8');

  assert.match(html, /x-for="g in diskGroups"/);
  assert.match(html, /:hidden="!\(diskGroups\.length === 0\)"/);
  assert.match(html, /class="storage-disk-tree"/);
  assert.match(html, /x-for="p in g\.partitions"/);
  assert.match(html, /media\/icons\/storage\/ufs_chip\.svg/);
  assert.doesNotMatch(html, /x-for="d in sysDisks"/);
});
