// utils.js — helpers purs, zéro dépendance

function fmtFreq(mhz) {
  const v = Number(mhz);
  if (!Number.isFinite(v) || v === 0) return '—';
  return v >= 1000 ? (v / 1000).toFixed(2) + ' GHz' : Math.round(v) + ' MHz';
}

function fmtMB(mb) {
  const v = Number(mb);
  if (!Number.isFinite(v)) return '—';
  return v >= 1024 ? (v / 1024).toFixed(1) + ' GiB' : Math.round(v) + ' MiB';
}

function formatUptime(str) {
  const secs = parseInt(String(str).split(' ')[0], 10);
  if (!Number.isFinite(secs)) return '—';
  let r = secs;
  const d = Math.floor(r / 86400); r %= 86400;
  const h = Math.floor(r / 3600); r %= 3600;
  const m = Math.floor(r / 60);
  const s = r % 60;
  const parts = [];
  if (d) parts.push(d + 'd');
  if (h) parts.push(h + 'h');
  if (m) parts.push(m + 'm');
  if (s || !parts.length) parts.push(s + 's');
  return parts.join(' ');
}

function cleanCPU(model) {
  if (!model) return '—';
  return String(model)
    .replace(/Intel\(R\)\s*/gi, '')
    .replace(/Core\(TM\)\s*/gi, '')
    .replace(/\s+CPU\s*/gi, ' ')
    .replace(/Processor/gi, '')
    .trim();
}

function osLogo(os) {
  const map = {
    alpine: '/media/img/logos/alpinelinux_logo_icon.png',
    debian: 'https://upload.wikimedia.org/wikipedia/commons/0/04/Debian_logo.svg',
    ubuntu: 'https://upload.wikimedia.org/wikipedia/commons/9/9e/Ubuntu_logo.svg',
    arch: 'https://upload.wikimedia.org/wikipedia/commons/a/a5/Archlinux-icon-crystal-64.svg',
    fedora: 'https://upload.wikimedia.org/wikipedia/commons/3/3f/Fedora_logo.svg',
  };
  const k = String(os || '').toLowerCase();
  for (const [key, url] of Object.entries(map)) {
    if (k.includes(key)) return url;
  }
  return '';
}

function cpuLogo(vendor) {
  if (/intel/i.test(vendor)) return '/media/img/logos/intel_logo_icon.png';
  if (/amd/i.test(vendor)) return '/media/img/logos/amd_logo_icon.png';
  return '';
}

function appIcon(type) {
  const map = {
    terminal: 'bi bi-terminal-fill',
    database: 'fa-solid fa-database',
    code: 'fa-solid fa-file-code',
    editor: 'fa-solid fa-file-code',
  };
  return map[type] || 'fa-solid fa-circle';
}