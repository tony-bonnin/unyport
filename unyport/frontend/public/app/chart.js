// chart.js — charts live UnyPort

const _MONO = 'JetBrains Mono, monospace';
const _GRID = 'rgba(255,255,255,0.04)';
const _TICK = { color: '#4a4a6a', font: { family: _MONO, size: 9 } };
const _RT = { duration: 30000, refresh: 2000, delay: 1000 };

let _netGauge = null;
let _freq = null;
let _mem = null;

// Couleurs réseau
const _RX_COLOR = 'rgba(236,72,153,0.85)';   // rose
const _TX_COLOR = 'rgba(59,130,246,0.85)';    // bleu
const _RX_LABEL = 'rgba(236,72,153,1)';
const _TX_LABEL = 'rgba(59,130,246,1)';

function initCharts() {
  _initNetGauge();
  _initFreq();
  _initMem();
}

function _initNetGauge() {
  const ctx = document.getElementById('netGauge')?.getContext('2d');
  if (!ctx || _netGauge) return;
  _netGauge = new Chart(ctx, {
    type: 'bar',
    data: {
      labels: ['↓ RX', '↑ TX'],
      datasets: [{
        data: [0, 0],
        backgroundColor: [_RX_COLOR, _TX_COLOR],
        borderColor: [_RX_LABEL, _TX_LABEL],
        borderWidth: 1,
        borderRadius: 3,
      }],
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      animation: { duration: 300 },
      scales: {
        x: { grid: { color: _GRID }, ticks: _TICK },
        y: { min: 0, max: 100, grid: { color: _GRID }, ticks: { display: false } },
      },
      plugins: { legend: { display: false }, tooltip: { enabled: false } },
    },
    plugins: [{
      id: 'netLabels',
      afterDatasetsDraw(chart) {
        const { ctx: c, scales } = chart;
        const labels = chart._netLabels || ['0 B/s', '0 B/s'];
        chart.getDatasetMeta(0).data.forEach((bar, i) => {
          c.save();
          c.fillStyle = '#484858';
          c.font = 'bold 8px monospace';
          c.textAlign = 'center';
          c.textBaseline = 'top';
          c.fillText(labels[i], bar.x, scales.y.bottom + 2);
          c.restore();
        });
      },
    }],
  });
}

function _initFreq() {
  const ctx = document.getElementById('chartFreq')?.getContext('2d');
  if (!ctx || _freq) return;
  _freq = new Chart(ctx, {
    type: 'line',
    data: { datasets: [{ data: [], borderColor: '#7b78ff', backgroundColor: 'rgba(123,120,255,0.08)', tension: 0.3, fill: true, pointRadius: 0, borderWidth: 1.5 }] },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      animation: false,
      scales: {
        x: { type: 'realtime', realtime: _RT, grid: { color: _GRID }, ticks: _TICK },
        y: { beginAtZero: false, grace: '50%', grid: { color: _GRID }, ticks: { ..._TICK, maxTicksLimit: 4, callback: v => v >= 1000 ? (v / 1000).toFixed(1) + ' GHz' : v + ' MHz' } },
      },
      plugins: { legend: { display: false } },
    },
  });
}

function _initMem() {
  const ctx = document.getElementById('chartMem')?.getContext('2d');
  if (!ctx || _mem) return;
  _mem = new Chart(ctx, {
    type: 'line',
    data: { datasets: [{ data: [], borderColor: '#22c9a0', backgroundColor: 'rgba(34,201,160,0.08)', tension: 0.3, fill: true, pointRadius: 0, borderWidth: 1.5 }] },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      animation: false,
      scales: {
        x: { type: 'realtime', realtime: _RT, grid: { color: _GRID }, ticks: _TICK },
        y: {
          beginAtZero: true, grid: { color: _GRID }, ticks: {
            ..._TICK, maxTicksLimit: 4, callback: function (v, i, ticks) {
              const max = ticks[ticks.length - 1]?.value ?? v;
              if (max >= 1024) {
                const gib = v / 1024;
                return (gib % 1 === 0 ? gib.toFixed(0) : gib.toFixed(1)) + ' GiB';
              }
              return Math.round(v) + ' MiB';
            }
          }
        },
      },
      plugins: { legend: { display: false } },
    },
  });
}

function updateGauge(pct) { }

// ── Réseau ──
let _maxRx = 0;
let _maxTx = 0;

function _updateNetGauge(component, rxBytes, txBytes) {
  if (rxBytes > _maxRx) _maxRx = rxBytes;
  if (txBytes > _maxTx) _maxTx = txBytes;

  // Si première valeur, initialise le max
  if (_maxRx === 0) _maxRx = rxBytes || 1;
  if (_maxTx === 0) _maxTx = txBytes || 1;

  const rxPct = Math.min(100, (rxBytes / _maxRx) * 100);
  const txPct = Math.min(100, (txBytes / _maxTx) * 100);

  if (_netGauge) {
    _netGauge._netLabels = [fmtNetSpeed(rxBytes), fmtNetSpeed(txBytes)];
    _netGauge.data.datasets[0].data = [rxPct, txPct];
    _netGauge.update();
  }

  component.netRxVal = fmtNetSpeed(rxBytes);
  component.netTxVal = fmtNetSpeed(txBytes);
}

function updateNetGauge() { }

function fmtNetSpeed(bps) {
  if (!Number.isFinite(bps) || bps <= 0) return '0 B/s';
  if (bps >= 1e9) return (bps / 1e9).toFixed(2) + ' GB/s';
  if (bps >= 1e6) return (bps / 1e6).toFixed(1) + ' MB/s';
  if (bps >= 1e3) return (bps / 1e3).toFixed(0) + ' KB/s';
  return Math.round(bps) + ' B/s';
}

function pushFreq(mhz) { _push(_freq, mhz); }
function pushMem(mb) { _push(_mem, mb); }

function _push(chart, value) {
  if (!chart || !chart.data?.datasets) return;
  if (!Number.isFinite(Number(value))) return;
  try {
    chart.data.datasets[0].data.push({ x: Date.now(), y: Number(value) });
    chart.update('none');
  } catch (_) { }
}

function destroyCharts() {
  [_freq, _mem, _netGauge].forEach(c => { try { c?.destroy(); } catch (_) { } });
  _freq = _mem = _netGauge = null;
}