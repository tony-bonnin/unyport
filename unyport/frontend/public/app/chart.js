// chart.js — gauge CPU + courbes freq/mem

const _MONO = 'JetBrains Mono, monospace';
const _GRID = 'rgba(255,255,255,0.04)';
const _TICK = { color: '#4a4a6a', font: { family: _MONO, size: 9 } };
const _RT = { duration: 30000, refresh: 2000, delay: 0 };

let _gauge = null;
let _freq = null;
let _mem = null;

function initCharts() {
  // Gauge CPU
  const ctxG = document.getElementById('cpuGauge')?.getContext('2d');
  if (ctxG && !_gauge) {
    _gauge = new Chart(ctxG, {
      type: 'doughnut',
      data: { datasets: [{ data: [0, 0, 0, 100], backgroundColor: ['rgba(123,120,255,0.9)', 'rgba(255,158,59,0.9)', 'rgba(255,68,68,0.9)', 'rgba(255,255,255,0.06)'], borderWidth: 0, cutout: '68%' }] },
      options: { responsive: true, animation: false, rotation: -135, circumference: 270, plugins: { legend: { display: false }, tooltip: { enabled: false } } },
    });
  }

  // Courbe fréquence
  const ctxF = document.getElementById('chartFreq')?.getContext('2d');
  if (ctxF && !_freq) {
    _freq = new Chart(ctxF, {
      type: 'line',
      data: { datasets: [{ data: [], borderColor: '#7b78ff', backgroundColor: 'rgba(123,120,255,0.08)', tension: 0.3, fill: true, pointRadius: 0, borderWidth: 1.5 }] },
      options: { responsive: true, animation: false, scales: { x: { type: 'realtime', realtime: _RT, grid: { color: _GRID }, ticks: _TICK }, y: { beginAtZero: false, grid: { color: _GRID }, ticks: { ..._TICK, callback: v => v >= 1000 ? (v / 1000).toFixed(1) + ' GHz' : v + ' MHz' } } }, plugins: { legend: { display: false } } },
    });
  }

  // Courbe mémoire
  const ctxM = document.getElementById('chartMem')?.getContext('2d');
  if (ctxM && !_mem) {
    _mem = new Chart(ctxM, {
      type: 'line',
      data: { datasets: [{ data: [], borderColor: '#22c9a0', backgroundColor: 'rgba(34,201,160,0.08)', tension: 0.3, fill: true, pointRadius: 0, borderWidth: 1.5 }] },
      options: { responsive: true, animation: false, scales: { x: { type: 'realtime', realtime: _RT, grid: { color: _GRID }, ticks: _TICK }, y: { beginAtZero: true, grid: { color: _GRID }, ticks: { ..._TICK, callback: v => v >= 1024 ? (v / 1024).toFixed(1) + ' GiB' : v + ' MiB' } } }, plugins: { legend: { display: false } } },
    });
  }
}

function updateGauge(pct) {
  if (!_gauge) return;
  const u = Math.max(0, Math.min(100, Math.round(pct)));
  _gauge.data.datasets[0].data = [Math.min(u, 33), u > 33 ? Math.min(u - 33, 33) : 0, u > 66 ? u - 66 : 0, 100 - u];
  _gauge.update();
}

function pushFreq(mhz) { _pushChart(_freq, mhz); }
function pushMem(mb) { _pushChart(_mem, mb); }

function _pushChart(chart, value) {
  if (!chart || !Number.isFinite(Number(value))) return;
  chart.data.datasets[0].data.push({ x: Date.now(), y: Number(value) });
  chart.update('none');
}

function destroyCharts() {
  [_gauge, _freq, _mem].forEach(c => c?.destroy());
  _gauge = _freq = _mem = null;
}