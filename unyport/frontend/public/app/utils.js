// utils.js — TRINITY · Xen/Alpine/DDM only
// Aucune dépendance externe

// ── Formatage ──────────────────────────────────────────────

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

function fmtBytes(b) {
  const v = Number(b);
  if (!Number.isFinite(v) || v <= 0) return '—';
  if (v >= 1e12) return (v / 1e12).toFixed(2) + ' TiB';
  if (v >= 1e9) return (v / 1e9).toFixed(2) + ' GiB';
  if (v >= 1e6) return (v / 1e6).toFixed(1) + ' MiB';
  if (v >= 1e3) return (v / 1e3).toFixed(0) + ' KiB';
  return Math.round(v) + ' B';
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
  if (d) parts.push(d + 'j');
  if (h) parts.push(h + 'h');
  if (m) parts.push(m + 'm');
  if (s || !parts.length) parts.push(s + 's');
  return parts.join(' ');
}

// Nettoie le modèle CPU pour l'affichage
function cleanCPU(model) {
  if (!model) return '—';
  return String(model)
    .replace(/Intel\(R\)\s*/gi, '')
    .replace(/Core\(TM\)\s*/gi, '')
    .replace(/\s+CPU\s*/gi, ' ')
    .replace(/Processor/gi, '')
    .trim();
}

// ── Xen ────────────────────────────────────────────────────

// Rôle Xen → label lisible
function xenRoleLabel(role) {
  if (role === 'Dom0') return 'Xen Dom0 · Hyperviseur';
  if (role === 'DomU') return 'Xen DomU · VM';
  return '—';
}

// Badge couleur selon état VM
function vmStatusClass(status) {
  const s = String(status || '').toLowerCase();
  if (s === 'running') return 'status-running';
  if (s === 'halted' || s === 'stopped') return 'status-halted';
  if (s === 'error' || s === 'crashed') return 'status-error';
  return 'status-unknown';
}

// ── Apps proxy ─────────────────────────────────────────────

function appIcon(type) {
  const map = {
    terminal: 'bi bi-terminal-fill',
    database: 'fa-solid fa-database',
    code: 'fa-solid fa-file-code',
    editor: 'fa-solid fa-file-code',
    web: 'fa-solid fa-globe',
    monitor: 'fa-solid fa-chart-line',
  };
  return map[String(type)] || 'fa-solid fa-circle-nodes';
}

// ── LBU ────────────────────────────────────────────────────

// Retourne true si le statut LBU indique des changements non commités
function lbuDirty(status) {
  return String(status || '').toLowerCase() !== 'clean';
}