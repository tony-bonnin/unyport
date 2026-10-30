// disk-groups.js | group the per-partition storage payload by physical disk
(function () {
  'use strict';

  function num(value) {
    var n = Number(value);
    return Number.isFinite(n) && n > 0 ? n : 0;
  }

  // Mirror of the backend rootBlockDevice helper, used as a fallback when an
  // older payload does not carry the parent `disk` field.
  function parentDevice(device) {
    var name = String(device == null ? '' : device).split('/').pop().trim();
    if (!name) return '';
    if (name.indexOf('nvme') === 0 || name.indexOf('mmcblk') === 0) {
      var p = name.lastIndexOf('p');
      if (p > 0 && /^\d+$/.test(name.slice(p + 1))) return name.slice(0, p);
      return name;
    }
    return name.replace(/\d+$/, '') || name;
  }

  function group(disks) {
    var list = Array.isArray(disks) ? disks : [];
    var out = [];
    var index = Object.create(null);
    list.forEach(function (disk) {
      if (!disk || typeof disk !== 'object') return;
      var key = String(disk.disk || '').trim() || parentDevice(disk.device);
      if (!key) return;
      var item = index[key];
      if (!item) {
        item = index[key] = {
          key: key,
          device: key,
          model: '',
          disk_type: '',
          transport: '',
          icon: '',
          pal: '',
          total_mb: 0,
          used_mb: 0,
          free_mb: 0,
          used_pct: 0,
          pct_cls: 'pct-0',
          bar_cls: '',
          partitions: []
        };
        out.push(item);
      }
      item.model = item.model || String(disk.model || '');
      item.disk_type = item.disk_type || String(disk.disk_type || '');
      item.transport = item.transport || String(disk.transport || '');
      item.icon = item.icon || String(disk.icon || '');
      item.pal = item.pal || String(disk.pal || '');
      item.total_mb += num(disk.total_mb);
      item.used_mb += num(disk.used_mb);
      item.free_mb += num(disk.free_mb);
      item.partitions.push(disk);
    });
    out.forEach(function (item) {
      item.used_pct = item.total_mb > 0 ? Math.min(100, Math.round((item.used_mb * 100) / item.total_mb)) : 0;
      item.pct_cls = 'pct-' + item.used_pct;
      item.bar_cls = item.used_pct > 75 ? 'crit' : (item.used_pct > 50 ? 'warn' : '');
      item.partitions.forEach(function (partition) {
        partition.used_pct = Math.min(100, Math.max(0, Math.round(num(partition.used_pct))));
        partition.pct_cls = 'pct-' + partition.used_pct;
      });
      item.volume_count = item.partitions.length;
      item.volume_label = item.partitions.length + (item.partitions.length === 1 ? ' volume' : ' volumes');
    });
    return out;
  }

  // Tooltip text that omits every empty field instead of emitting the legacy
  // "|" placeholder separator.
  function tooltip(disk, formatMB) {
    if (!disk || typeof disk !== 'object') return '';
    var fmt = typeof formatMB === 'function'
      ? formatMB
      : function (mb) { return num(mb) + ' MB'; };
    var parts = [];
    [disk.device, disk.mount, disk.fstype].forEach(function (value) {
      var text = String(value == null ? '' : value).trim();
      if (text) parts.push(text);
    });
    var total = num(disk.total_mb);
    if (total > 0) {
      parts.push(fmt(num(disk.used_mb)) + ' / ' + fmt(total) + ' (' + num(disk.used_pct) + '%)');
    }
    var free = num(disk.free_mb);
    if (free > 0) parts.push(fmt(free) + ' free');
    var model = String(disk.model || '').trim();
    if (model) parts.push(model);
    return parts.join(' · ');
  }

  var api = { group: group, parentDevice: parentDevice, tooltip: tooltip };
  if (typeof window !== 'undefined') window.UnyPortDiskGroups = api;
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
})();
