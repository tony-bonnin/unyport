// security-charts.js | compact Chart.js views for the Security page
(function () {
  'use strict';

  var charts = {};

  function css(name, fallback) {
    var value = window.getComputedStyle ? getComputedStyle(document.documentElement).getPropertyValue(name).trim() : '';
    return value || fallback;
  }

  function primary(alpha) {
    var value = css('--brand', '#3e3aab');
    var hex = value.match(/^#([\da-f]{6})$/i);
    if (hex) {
      var color = parseInt(hex[1], 16);
      return 'rgba(' + ((color >> 16) & 255) + ',' + ((color >> 8) & 255) + ',' + (color & 255) + ',' + alpha + ')';
    }
    return value;
  }

  function chartPalette() {
    return [primary(1), primary(.58), primary(.3), primary(.12)];
  }

  function animation(kind) {
    var reduced = window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    if (reduced) return { duration: 0 };
    if (kind === 'arc') {
      return { duration: 950, easing: 'easeOutQuart', animateRotate: true, animateScale: true };
    }
    return {
      duration: 950,
      easing: 'easeOutQuart',
      delay: function (context) {
        return context.type === 'data' ? context.dataIndex * 90 + context.datasetIndex * 45 : 0;
      }
    };
  }

  function nonZeroSegments(labels, values) {
    var palette = chartPalette();
    var segments = labels.map(function (label, index) {
      return { label: label, value: Number(values[index]) || 0, color: palette[index] };
    }).filter(function (segment) { return segment.value > 0; });
    if (!segments.length) segments.push({ label: 'Unknown', value: 1, color: primary(.12) });
    return {
      labels: segments.map(function (segment) { return segment.label; }),
      values: segments.map(function (segment) { return segment.value; }),
      colors: segments.map(function (segment) { return segment.color; })
    };
  }

  function destroy(name) {
    if (charts[name]) charts[name].destroy();
    charts[name] = null;
  }

  function chart(name, config) {
    var canvas = document.getElementById(name);
    destroy(name);
    if (!canvas || typeof window.Chart === 'undefined') return;
    charts[name] = new window.Chart(canvas, config);
    canvas.classList.remove('chart-load-enter');
    void canvas.offsetWidth;
    canvas.classList.add('chart-load-enter');
  }

  function commonLegend() {
    return { position: 'bottom', labels: { usePointStyle: true, pointStyle: 'circle', boxWidth: 7, boxHeight: 7, padding: 14, color: css('--text-dim', '#7b8192'), font: { size: 10, weight: '600' } } };
  }

  function statusChart(summary) {
    var segments = nonZeroSegments(
      ['Passed', 'Warnings', 'Critical', 'Unknown'],
      [summary.ok || 0, summary.warn || 0, summary.critical || 0, summary.unknown || 0]
    );
    chart('security-posture-chart', {
      type: 'doughnut',
      data: {
        labels: segments.labels,
        datasets: [{ data: segments.values, backgroundColor: segments.colors, borderWidth: 0, spacing: 0, hoverOffset: 0 }]
      },
      options: { responsive: true, maintainAspectRatio: false, cutout: '76%', rotation: -175, circumference: 350, animation: animation('arc'), plugins: { legend: { display: false }, tooltip: { displayColors: false } } }
    });
  }

  function categoryChart(checks) {
    var grouped = Object.create(null);
    (checks || []).forEach(function (item) {
      var key = item.category || 'other';
      if (!grouped[key]) grouped[key] = { ok: 0, warn: 0, critical: 0, unknown: 0 };
      grouped[key][item.status || 'unknown']++;
    });
    var labels = Object.keys(grouped);
    chart('security-category-chart', {
      type: 'bar',
      data: {
        labels: labels,
        datasets: [
          { label: 'Passed', data: labels.map(function (key) { return grouped[key].ok; }), backgroundColor: primary(1), borderWidth: 0, borderRadius: 3 },
          { label: 'Warnings', data: labels.map(function (key) { return grouped[key].warn; }), backgroundColor: primary(.55), borderWidth: 0, borderRadius: 3 },
          { label: 'Critical', data: labels.map(function (key) { return grouped[key].critical; }), backgroundColor: primary(.22), borderWidth: 0, borderRadius: 3 }
        ]
      },
      options: {
        responsive: true, maintainAspectRatio: false, animation: animation('bar'),
        animations: { y: { from: function (context) { return context.chart.scales.y.getPixelForValue(0); } } },
        scales: {
          x: { stacked: true, grid: { display: false }, ticks: { color: css('--text-dim', '#7b8192'), font: { size: 10, weight: '600' } } },
          y: { stacked: true, beginAtZero: true, ticks: { precision: 0, color: css('--text-dim', '#7b8192'), font: { size: 10 } }, grid: { color: css('--border', '#e4e7ef') } }
        },
        plugins: { legend: commonLegend(), tooltip: { mode: 'index', intersect: false } }
      }
    });
  }

  function serviceChart(services) {
    var counts = { ok: 0, warn: 0, critical: 0, unknown: 0 };
    (services || []).forEach(function (service) { counts[service.status] = (counts[service.status] || 0) + 1; });
    var segments = nonZeroSegments(
      ['Healthy', 'Warning', 'Critical', 'Unknown'],
      [counts.ok, counts.warn, counts.critical, counts.unknown]
    );
    chart('security-service-chart', {
      type: 'doughnut',
      data: {
        labels: segments.labels,
        datasets: [{ data: segments.values, backgroundColor: segments.colors, borderWidth: 0, spacing: 0, hoverOffset: 0 }]
      },
      options: { responsive: true, maintainAspectRatio: false, cutout: '68%', rotation: -175, circumference: 350, animation: animation('arc'), plugins: { legend: commonLegend() } }
    });
  }

  function render(payload) {
    requestAnimationFrame(function () {
      statusChart(payload.summary || {});
      categoryChart(payload.checks || []);
      serviceChart(payload.services || []);
    });
  }

  window.UnyPortSecurityCharts = { render: render, destroy: function () { Object.keys(charts).forEach(destroy); } };
}());
