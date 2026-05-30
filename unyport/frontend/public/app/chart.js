// chart.js — TRINITY · oscilloscopes + Network Map HTML
// #chartFreq → Effective CPU frequency (MHz) = usage × freqMax / 100
// #chartMem  → Memory used (MiB)
//
// Go computes freq_effective from /proc/stat × freqMax → cpu_freq_avg_mhz
// Go computes oscilloscope scales → cpu_freq_y_min/max, mem_y_min/max
// JS: pushSnapshot writes _state, onRefresh reads _state + applies ys.options.min/max

const _MONO = 'JetBrains Mono, monospace';
const _GRID = 'rgba(0,0,0,0.04)';
const _TICK = { color: '#9aa0b0', font: { family: _MONO, size: 9 } };
const _DURATION = 30000;
const _REFRESH = 2000;
const _DELAY = 500;

function _brand(alpha) {
  const hex = getComputedStyle(document.documentElement)
    .getPropertyValue('--brand').trim() || '#3e3aab';
  const r = parseInt(hex.slice(1, 3), 16);
  const g = parseInt(hex.slice(3, 5), 16);
  const b = parseInt(hex.slice(5, 7), 16);
  return `rgba(${r},${g},${b},${alpha})`;
}

const _state = {
  freqAvg: 0,
  freqYMin: 0,
  freqYMax: 1804,
  memUsedMb: 0,
  memYMin: 0,
  memYMax: 4096,
};

function _refreshFreq(chart) {
  if (_chartsSuspended || !chart?.canvas?.isConnected) return;
  chart.data.datasets[0].data.push({ x: Date.now(), y: _state.freqAvg });
  chart.data.datasets[0].borderColor = _brand(0.85);
  chart.data.datasets[0].backgroundColor = _brand(0.09);
  const yMin = _state.freqYMin;
  const yMax = _state.freqYMax;
  if (yMax > yMin) {
    const ys = chart.scales.y;
    if (ys) { ys.options.min = yMin; ys.options.max = yMax; }
  }
}

function _refreshMem(chart) {
  if (_chartsSuspended || !chart?.canvas?.isConnected) return;
  chart.data.datasets[0].data.push({ x: Date.now(), y: _state.memUsedMb });
  chart.data.datasets[0].borderColor = _brand(0.85);
  chart.data.datasets[0].backgroundColor = _brand(0.09);
  const yMin = _state.memYMin;
  const yMax = _state.memYMax;
  if (yMax > yMin) {
    const ys = chart.scales.y;
    if (ys) { ys.options.min = yMin; ys.options.max = yMax; }
  }
}

let _freqChart = null;
let _memChart = null;
let _initTimer = null;
let _chartsSuspended = false;

// initCharts — appelé depuis app.js après $nextTick.
// Les canvas existent dans le DOM mais peuvent être dans une section masquée
// (x-show → display:none). chartjs-streaming tente de styler les datasets
// avant que le canvas soit rendu → _setStyle crash.
// Solution : observer la visibilité du canvas, init seulement quand visible.
function initCharts() {
  _chartsSuspended = false;
  _resumeChart(_freqChart);
  _resumeChart(_memChart);
  _tryInit();
}

function _tryInit() {
  const elFreq = document.getElementById('chartFreq');
  const elMem = document.getElementById('chartMem');

  if (!elFreq || !elMem) { _scheduleRetry(); return; }
  if (elFreq.offsetParent === null || elMem.offsetParent === null) { _scheduleRetry(); return; }

  _initFreq(elFreq);
  _initMem(elMem);
}

function _scheduleRetry() {
  if (_initTimer) return;
  _initTimer = setTimeout(() => { _initTimer = null; _tryInit(); }, 300);
}

function _initFreq(el) {
  if (_freqChart) return;
  const ctx = el.getContext('2d');
  if (!ctx) return;

  _freqChart = new Chart(ctx, {
    type: 'line',
    data: {
      datasets: [{
        data: [],
        borderColor: _brand(0.85), backgroundColor: _brand(0.09),
        borderWidth: 1.5, tension: 0.3, fill: true, pointRadius: 0,
      }]
    },
    options: {
      responsive: true, maintainAspectRatio: false, animation: false,
      scales: {
        x: {
          type: 'realtime',
          realtime: {
            duration: _DURATION, refresh: _REFRESH, delay: _DELAY,
            onRefresh: _refreshFreq,
          },
          grid: { color: _GRID }, ticks: _TICK,
        },
        y: {
          grid: { color: _GRID },
          ticks: {
            ..._TICK, maxTicksLimit: 4,
            callback: v => v >= 1000
              ? (v / 1000).toFixed(2) + ' GHz'
              : Math.round(v) + ' MHz',
          },
        },
      },
      plugins: { legend: { display: false } },
    },
  });
}

function _initMem(el) {
  if (_memChart) return;
  const ctx = el.getContext('2d');
  if (!ctx) return;

  _memChart = new Chart(ctx, {
    type: 'line',
    data: {
      datasets: [{
        data: [],
        borderColor: _brand(0.85), backgroundColor: _brand(0.09),
        borderWidth: 1.5, tension: 0.3, fill: true, pointRadius: 0,
      }]
    },
    options: {
      responsive: true, maintainAspectRatio: false, animation: false,
      scales: {
        x: {
          type: 'realtime',
          realtime: {
            duration: _DURATION, refresh: _REFRESH, delay: _DELAY,
            onRefresh: _refreshMem,
          },
          grid: { color: _GRID }, ticks: _TICK,
        },
        y: {
          grid: { color: _GRID },
          ticks: {
            ..._TICK, maxTicksLimit: 4,
            callback: v => v >= 1024
              ? (v / 1024).toFixed(2) + ' GiB'
              : Math.round(v) + ' MiB',
          },
        },
      },
      plugins: { legend: { display: false } },
    },
  });
}

// ── pushSnapshot ──────────────────────────────────────────────────────────────
// Go sends cpu_freq_avg_mhz = usage% × freqMax / 100 (from /proc/stat)
// and oscilloscope scales: cpu_freq_y_min/max, mem_y_min/max
function pushSnapshot(snap) {
  if (!snap) return;
  _state.freqAvg = Number(snap.cpu_freq_avg_mhz) || 0;
  _state.freqYMin = Number(snap.cpu_freq_y_min) || 0;
  _state.freqYMax = Number(snap.cpu_freq_y_max) || 1804;
  _state.memUsedMb = Number(snap.mem_used_mb) || 0;
  _state.memYMin = Number(snap.mem_y_min) || 0;
  _state.memYMax = Number(snap.mem_y_max) || Number(snap.mem_total_mb) || 4096;

  // Si les charts n'ont pas encore pu s'initialiser (section masquée),
  // réessayer à chaque snapshot reçu.
  if (!_freqChart || !_memChart) _tryInit();
}

// ── Network gauge ─────────────────────────────────────────────────────────────
function _updateNetGauge(component, rxBps, txBps) {
  component.netRxVal = _fmtBps(rxBps);
  component.netTxVal = _fmtBps(txBps);
}

function _fmtBps(bps) {
  if (!Number.isFinite(bps) || bps <= 0) return '0 B/s';
  if (bps >= 1e9) return (bps / 1e9).toFixed(2) + ' GB/s';
  if (bps >= 1e6) return (bps / 1e6).toFixed(1) + ' MB/s';
  if (bps >= 1e3) return (bps / 1e3).toFixed(0) + ' KB/s';
  return Math.round(bps) + ' B/s';
}

// ── Destroy ───────────────────────────────────────────────────────────────────
function destroyCharts() {
  if (_initTimer) { clearTimeout(_initTimer); _initTimer = null; }
  _chartsSuspended = true;
  [_freqChart, _memChart].forEach(_suspendChart);
}

function _suspendChart(chart) {
  if (!chart) return;
  try {
    const rt = chart.options?.scales?.x?.realtime;
    if (rt) {
      rt.pause = true;
      rt.onRefresh = () => { };
    }
    chart.stop();
  } catch (_) { }
}

function _resumeChart(chart) {
  if (!chart) return;
  try {
    const rt = chart.options?.scales?.x?.realtime;
    if (rt) {
      rt.pause = false;
      if (chart === _freqChart) rt.onRefresh = _refreshFreq;
      if (chart === _memChart) rt.onRefresh = _refreshMem;
    }
    chart.update('none');
  } catch (_) { }
}

// Stubs kept for compat
function updateGauge() { }
function updateNetGauge() { }
function pushFreq() { }
function pushMem() { }

// ═══════════════════════════════════════════════════════════════════════════════
// NETWORK MAP — HTML pur, zéro SVG
// ─────────────────────────────────────────────────────────────────────────────
// Layout hiérarchique : host → interfaces → voisins ARP
// Tooltip flottant au survol de chaque node (positionné via CSS custom props)
// CSP-safe : zéro inline style, zéro eval, classes CSS uniquement
// ═══════════════════════════════════════════════════════════════════════════════

function _fmtBpsMap(b) {
  if (!b || b <= 0) return '0 B/s';
  if (b >= 1e6) return (b / 1e6).toFixed(1) + ' MB/s';
  if (b >= 1e3) return (b / 1e3).toFixed(0) + ' KB/s';
  return Math.round(b) + ' B/s';
}

// ── Tooltip singleton ─────────────────────────────────────────────────────────
// Un seul .nm-tooltip dans le body, repositionné par JS à chaque survol.
let _nmTooltipEl = null;

function _getNmTooltip() {
  if (_nmTooltipEl && document.body.contains(_nmTooltipEl)) return _nmTooltipEl;
  _nmTooltipEl = document.createElement('div');
  _nmTooltipEl.className = 'nm-tooltip';
  _nmTooltipEl.setAttribute('aria-hidden', 'true');
  document.body.appendChild(_nmTooltipEl);
  return _nmTooltipEl;
}

function _positionTooltip(node) {
  const tt = _getNmTooltip();
  const rect = node.getBoundingClientRect();
  const scrollY = window.scrollY || 0;
  const scrollX = window.scrollX || 0;
  const ttW = 230;
  let left = rect.left + scrollX + rect.width / 2 - ttW / 2;
  let top = rect.top + scrollY - 8; // au-dessus
  left = Math.max(8, Math.min(left, window.innerWidth + scrollX - ttW - 8));
  tt.style.setProperty('--nm-tt-x', left + 'px');
  tt.style.setProperty('--nm-tt-y', top + 'px');
}

function _showNmTooltip(node, data) {
  const tt = _getNmTooltip();
  let rows = '';
  for (const [k, v] of Object.entries(data)) {
    if (v === undefined || v === null || v === '' || v === '—') continue;
    rows += `<div class="nm-tt-row"><span class="nm-tt-key">${k}</span><span class="nm-tt-val">${v}</span></div>`;
  }
  tt.innerHTML = `<div class="nm-tt-inner">${rows}</div>`;
  _positionTooltip(node);
  tt.classList.add('nm-tooltip-visible');
}

function _hideNmTooltip() {
  if (_nmTooltipEl) _nmTooltipEl.classList.remove('nm-tooltip-visible');
}

// ── Construction d'un nœud HTML ───────────────────────────────────────────────
function _nmNode(type, icon, label, sub, detail, extraClass, tooltipData) {
  const el = document.createElement('div');
  el.className = 'nm-node nm-node-' + type + (extraClass ? ' ' + extraClass : '');

  el.innerHTML =
    '<div class="nm-node-ico"><i class="' + icon + '"></i></div>' +
    '<div class="nm-node-info">' +
    '<div class="nm-node-title">' + label + '</div>' +
    (sub ? '<div class="nm-node-sub">' + sub + '</div>' : '') +
    (detail ? '<div class="nm-node-detail">' + detail + '</div>' : '') +
    '</div>';

  el.addEventListener('mouseenter', function () { _showNmTooltip(el, tooltipData); });
  el.addEventListener('mouseleave', _hideNmTooltip);
  el.addEventListener('mousemove', function () { _positionTooltip(el); });

  return el;
}

// ── Connecteur vertical ───────────────────────────────────────────────────────
function _nmConn(cls) {
  const el = document.createElement('div');
  el.className = cls || 'nm-conn';
  return el;
}

// ── renderNetworkMap — point d'entrée ─────────────────────────────────────────
function renderNetworkMap(netMap, hostRole, hostRuntime, hostName, hostIP, containerId) {
  const el = document.getElementById(containerId || 'netmap-container');
  if (!el) return;

  const ifaces = (netMap && netMap.interfaces) ? netMap.interfaces : [];
  const neighbors = (netMap && netMap.neighbors) ? netMap.neighbors : [];

  // ── Vue mobile : liste simple ─────────────────────────────
  if (window.innerWidth < 600) {
    el.innerHTML = '<div class="nm-list">' +
      ifaces.map(function (iface) {
        const nb = neighbors.filter(function (n) { return n.iface === iface.name; });
        return '<div class="nm-list-iface">' +
          '<span class="nm-list-name">' + iface.name + '</span>' +
          '<span class="nm-list-ip">' + (iface.ip || '—') + '</span>' +
          nb.map(function (n) {
            return '<div class="nm-list-nb"><span>' + n.ip + '</span>' +
              '<span class="nm-list-state nm-state-' + n.state + '">' + n.state + '</span></div>';
          }).join('') +
          '</div>';
      }).join('') + '</div>';
    return;
  }

  if (ifaces.length === 0) {
    el.innerHTML = '<p class="nm-empty">No interface data yet…</p>';
    return;
  }

  // ── Classe rôle host ──────────────────────────────────────
  const roleClassMap = { Dom0: 'nm-role-dom0', DomU: 'nm-role-domu', Container: 'nm-role-container', Alpine: 'nm-role-alpine' };
  const roleClass = roleClassMap[hostRole] || 'nm-role-alpine';

  const hostIcon =
    hostRuntime === 'docker' ? 'fa-brands fa-docker' :
      hostRuntime === 'podman' ? 'fa-solid fa-box' :
        hostRole === 'Dom0' ? 'fa-solid fa-server' :
          hostRole === 'DomU' ? 'fa-solid fa-display' :
            hostRole === 'Alpine' ? 'fa-solid fa-mountain-sun' :
              hostRole === 'Container' ? 'fa-solid fa-box' :
                'fa-solid fa-server';

  // ── Racine ────────────────────────────────────────────────
  const wrap = document.createElement('div');
  wrap.className = 'nm-tree';

  // ── Niveau 0 : host ───────────────────────────────────────
  const hostRow = document.createElement('div');
  hostRow.className = 'nm-row-host';
  hostRow.appendChild(_nmNode(
    'host', hostIcon,
    hostName || 'host', hostIP || '—', hostRole || 'Alpine',
    roleClass,
    { Role: hostRole, IP: hostIP, Runtime: hostRuntime || '—', Hostname: hostName }
  ));
  wrap.appendChild(hostRow);
  wrap.appendChild(_nmConn('nm-conn nm-conn-host'));

  // ── Niveau 1 : interfaces ─────────────────────────────────
  const ifaceBar = document.createElement('div');
  ifaceBar.className = 'nm-iface-bar';

  ifaces.forEach(function (iface) {
    const nb = neighbors.filter(function (n) { return n.iface === iface.name; });
    const isUp = iface.up !== false;
    const rx = _fmtBpsMap(iface.rx_bps || 0);
    const tx = _fmtBpsMap(iface.tx_bps || 0);
    const rxTotal = iface.rx_bytes ? _fmtBytes(iface.rx_bytes) : '—';
    const txTotal = iface.tx_bytes ? _fmtBytes(iface.tx_bytes) : '—';

    const col = document.createElement('div');
    col.className = 'nm-col';

    col.appendChild(_nmConn('nm-conn nm-conn-iface-top'));

    col.appendChild(_nmNode(
      'iface', 'fa-solid fa-network-wired',
      iface.name, iface.ip || '—', '\u2193' + rx + ' \u2191' + tx,
      isUp ? 'nm-iface-up' : 'nm-iface-dn',
      {
        Interface: iface.name,
        IP: iface.ip || '—',
        Status: isUp ? 'UP' : 'DOWN',
        '↓ RX': rx,
        '↑ TX': tx,
        'RX total': rxTotal,
        'TX total': txTotal,
        MAC: iface.mac || '—',
      }
    ));

    // ── Niveau 2 : voisins ARP ────────────────────────────
    if (nb.length > 0) {
      col.appendChild(_nmConn('nm-conn nm-conn-nb-top'));

      const nbRow = document.createElement('div');
      nbRow.className = 'nm-nb-row';

      nb.forEach(function (n, ni) {
        if (ni > 0) {
          const sep = document.createElement('div');
          sep.className = 'nm-nb-sep';
          nbRow.appendChild(sep);
        }

        const alive = n.state === 'reachable';
        const stale = n.state === 'stale';
        const nbCls = alive ? 'nm-nb-alive' : stale ? 'nm-nb-stale' : 'nm-nb-unk';

        const nbWrap = document.createElement('div');
        nbWrap.className = 'nm-nb-wrap';
        nbWrap.appendChild(_nmConn('nm-conn nm-conn-nb'));
        nbWrap.appendChild(_nmNode(
          'neighbor', 'fa-solid fa-display',
          n.ip, (n.mac || '').substring(0, 17), n.state,
          nbCls,
          { IP: n.ip, MAC: n.mac || '—', Status: n.state, Via: n.iface || iface.name, Vendor: n.vendor || '—' }
        ));
        nbRow.appendChild(nbWrap);
      });

      col.appendChild(nbRow);
    }

    ifaceBar.appendChild(col);
  });

  wrap.appendChild(ifaceBar);

  el.innerHTML = '';
  el.appendChild(wrap);
}
