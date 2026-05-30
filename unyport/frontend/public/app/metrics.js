// metrics.js — TRINITY · DOM updates impératifs pour les pages métriques
// Toutes les fonctions ici manipulent le DOM directement (CSP-safe, zéro :style inline).
// Appelées depuis _onSnapshot() dans app.js à chaque tick SSE.

// ── Per-core grid (page Resources) ───────────────────────────────────────────
function _updatePerCoreGrid(perCore, freqCore) {
  const el = document.getElementById('per-core-grid');
  if (!el || !Array.isArray(perCore) || perCore.length === 0) return;
  el.innerHTML = perCore.map((pct, i) => {
    const freq = freqCore && freqCore[i] ? freqCore[i] : 0;
    const p = Math.round(Math.max(0, Math.min(100, pct)));
    const cls = p > 80 ? 'crit' : p > 60 ? 'warn' : '';
    const fStr = freq >= 1000 ? (freq / 1000).toFixed(2) + ' GHz' : freq + ' MHz';
    return `<div class="core-block" data-pct="${p}">
      <div class="core-label">C${i}</div>
      <div class="core-bar-wrap"><div class="core-bar ${cls}"></div></div>
      <div class="core-pct">${p}%</div>
      <div class="core-freq">${freq ? fStr : ''}</div>
    </div>`;
  }).join('');
  el.querySelectorAll('.core-block').forEach(block => {
    block.querySelector('.core-bar-wrap').style.setProperty('--core-w', (block.dataset.pct || '0') + '%');
  });
}

// ── Net ifaces grid (page Network) ───────────────────────────────────────────
function _updateNetIfacesGrid(netMap) {
  const el = document.getElementById('net-ifaces-grid');
  if (!el || !netMap || !Array.isArray(netMap.interfaces)) return;
  const ifaces = netMap.interfaces;
  if (ifaces.length === 0) return;
  el.innerHTML = ifaces.map(iface => {
    const rx = _fmtBpsMap(iface.rx_bps || 0);
    const tx = _fmtBpsMap(iface.tx_bps || 0);
    const up = iface.up !== false;
    return `<div class="res-card">
      <div class="res-card-head">
        <div class="res-icon ${up ? 'res-blue' : 'res-orange'}">
          <i class="fa-solid fa-network-wired"></i>
        </div>
        <h3>${iface.name}</h3>
        <span class="metric-badge ${up ? 'badge-up' : 'badge-down'}">${up ? 'UP' : 'DOWN'}</span>
      </div>
      <div class="res-net-row">
        <div class="res-net-block"><span>↓ RX</span><strong>${rx}</strong></div>
        <div class="res-net-block"><span>↑ TX</span><strong>${tx}</strong></div>
      </div>
      <dl class="res-kv">
        <dt>IP</dt><dd>${iface.ip || '—'}</dd>
        <dt>RX total</dt><dd>${iface.rx_bytes ? _fmtBytes(iface.rx_bytes) : '—'}</dd>
        <dt>TX total</dt><dd>${iface.tx_bytes ? _fmtBytes(iface.tx_bytes) : '—'}</dd>
      </dl>
    </div>`;
  }).join('');
}

// ── Disk bars ─────────────────────────────────────────────────────────────────
function _updateDiskBars(disks) {
  if (!Array.isArray(disks) || disks.length === 0) return;
  requestAnimationFrame(function () {
    document.querySelectorAll('.disk-bar-wrap').forEach(function (wrap, i) {
      var d = disks[i];
      if (!d) return;
      wrap.style.setProperty('--disk-w', Math.min(100, Math.max(0, d.used_pct || 0)) + '%');
    });
  });
}

// ── Température bars ──────────────────────────────────────────────────────────
function _updateTempBars(temps) {
  if (!Array.isArray(temps)) return;
  requestAnimationFrame(() => {
    document.querySelectorAll('.temp-row').forEach((row, i) => {
      const t = temps[i];
      if (t) row.style.setProperty('--temp-w', Math.min(100, t.temp_c) + '%');
    });
  });
}

// ── Htop panel ────────────────────────────────────────────────────────────────
function _updateHtop(snap) {
  const panel = document.querySelector('.htop-panel');
  if (!panel) return;

  const p = Math.round(Math.max(0, Math.min(100, snap.cpu_usage || 0)));
  panel.style.setProperty('--htop-cpu', p + '%');
  panel.style.setProperty('--htop-cpu-cls', p > 80 ? 'crit' : p > 60 ? 'warn' : 'ok');

  if (snap.mem_total_mb > 0) {
    const total = snap.mem_total_mb;
    const used = snap.mem_used_mb || 0;
    const cached = snap.mem_cached_mb || 0;
    const usedPct = Math.round((used / total) * 100);
    const cachePct = Math.round((cached / total) * 100);
    panel.style.setProperty('--htop-mem', usedPct + '%');
    panel.style.setProperty('--htop-cache-l', usedPct + '%');
    panel.style.setProperty('--htop-cache-w', Math.min(cachePct, 100 - usedPct) + '%');
  }

  const cpuBarEl = panel.querySelector('.htop-bar-cpu');
  if (cpuBarEl) cpuBarEl.className = 'htop-bar-cpu' + (p > 80 ? ' crit' : p > 60 ? ' warn' : '');

  const coresEl = document.getElementById('htop-cores');
  if (coresEl && Array.isArray(snap.cpu_per_core) && snap.cpu_per_core.length > 0) {
    coresEl.innerHTML = snap.cpu_per_core.map((pct, i) => {
      const cp = Math.round(Math.max(0, Math.min(100, pct)));
      const cls = cp > 80 ? 'crit' : cp > 60 ? 'warn' : '';
      return `<div class="htop-core-row" data-pct="${cp}">
        <span class="htop-core-label">C${i}</span>
        <div class="htop-core-bar-wrap">
          <div class="htop-core-bar ${cls}"></div>
        </div>
        <span class="htop-core-val">${cp}%</span>
      </div>`;
    }).join('');
    coresEl.querySelectorAll('.htop-core-row').forEach(row => {
      row.querySelector('.htop-core-bar-wrap').style.setProperty('--w', (row.dataset.pct || '0') + '%');
    });
  }
}

// ── Top processes — barres MEM% ───────────────────────────────────────────────
function _updateTopProcBars() {
  requestAnimationFrame(() => {
    document.querySelectorAll('.tp-mem-bar').forEach(bar => {
      const pct = parseFloat(bar.dataset.pct || bar.closest('tr')?.querySelector('[data-pct]')?.dataset.pct || 0);
      bar.style.setProperty('--tp-w', Math.min(100, Math.max(0, pct)) + '%');
      if (pct > 80) bar.style.setProperty('background', '#dc2626');
      else if (pct > 50) bar.style.setProperty('background', '#d97706');
      else bar.style.removeProperty('background');
    });
  });
}