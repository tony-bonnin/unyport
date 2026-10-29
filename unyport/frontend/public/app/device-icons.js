// device-icons.js | UnyPort device icon set
// SVG natif, sans police ni ressource externe. Chaque icône tient dans un viewBox 64x64.
(function () {
  'use strict';
  var NS = 'http://www.w3.org/2000/svg';
  var catalog = {
    desktop:     { label: 'PC',             color: '#72a7dc' },
    laptop:      { label: 'Portable',       color: '#8999df' },
    server:      { label: 'Serveur',        color: '#a58bd4' },
    nas:         { label: 'NAS',            color: '#c18bc3' },
    printer:     { label: 'Imprimante',     color: '#df8fb1' },
    phone:       { label: 'Téléphone',      color: '#e88989' },
    tablet:      { label: 'Tablette',       color: '#e9a06f' },
    router:      { label: 'Routeur',        color: '#d8b85f' },
    switch:      { label: 'Switch',         color: '#69b9ad' },
    accesspoint: { label: "Point d'accès", color: '#79bd88' },
    camera:      { label: 'Caméra',         color: '#8eae9f' },
    tv:          { label: 'Télévision',     color: '#75afd0' },
    iot:         { label: 'Objet connecté', color: '#aaa56e' },
    unknown:     { label: 'Équipement',     color: '#9da8b7' }
  };
  var aliases = { pc: 'desktop', computer: 'desktop', workstation: 'desktop', notebook: 'laptop', mobile: 'phone', smartphone: 'phone', print: 'printer', access_point: 'accesspoint', ap: 'accesspoint', wifi: 'accesspoint', television: 'tv', smarttv: 'tv', storage: 'nas', appliance: 'iot' };
  function el(tag, attrs) { var node = document.createElementNS(NS, tag); Object.keys(attrs || {}).forEach(function (key) { node.setAttribute(key, String(attrs[key])); }); return node; }
  function resolve(type) { var key = String(type || '').trim().toLowerCase().replace(/[ -]+/g, '_'); key = aliases[key] || key.replace(/_/g, ''); return catalog[key] ? key : 'unknown'; }
  function line(g, x1, y1, x2, y2, width) { g.appendChild(el('line', { x1: x1, y1: y1, x2: x2, y2: y2, stroke: '#fff', 'stroke-width': width || 2, 'stroke-linecap': 'round', 'stroke-linejoin': 'round' })); }
  function rect(g, x, y, w, h, rx, attrs) { var a = { x: x, y: y, width: w, height: h, rx: rx || 0, fill: '#fff' }; Object.keys(attrs || {}).forEach(function (key) { a[key] = attrs[key]; }); g.appendChild(el('rect', a)); }
  function drawInner(g, type, color) {
    if (type === 'desktop') { rect(g, 8, 8, 32, 22, 4); rect(g, 20, 30, 8, 6, 0); rect(g, 14, 36, 20, 3, 1); rect(g, 12, 12, 24, 14, 2, { fill: color, opacity: '.4' }); }
    else if (type === 'laptop') { rect(g, 10, 7, 28, 23, 4); rect(g, 13, 10, 22, 17, 2, { fill: color, opacity: '.4' }); g.appendChild(el('path', { d: 'M6 33h36l-4 6H10z', fill: '#fff' })); }
    else if (type === 'server' || type === 'nas') { rect(g, 9, 5, 30, 35, 5); [10, 20, 30].forEach(function (y) { rect(g, 13, y, 22, 6, 2, { fill: color, opacity: '.4' }); g.appendChild(el('circle', { cx: 17, cy: y + 3, r: 1.5, fill: color })); line(g, 22, y + 3, 31, y + 3, 1.5); }); }
    else if (type === 'printer') { rect(g, 13, 4, 22, 15, 3); rect(g, 8, 16, 32, 20, 5); rect(g, 13, 28, 22, 12, 2); rect(g, 16, 31, 16, 2, 1, { fill: color, opacity: '.55' }); g.appendChild(el('circle', { cx: 34, cy: 21, r: 2, fill: color })); }
    else if (type === 'phone') { rect(g, 15, 3, 18, 39, 5); rect(g, 18, 8, 12, 27, 2, { fill: color, opacity: '.4' }); g.appendChild(el('circle', { cx: 24, cy: 38, r: 1.8, fill: color })); }
    else if (type === 'tablet') { rect(g, 10, 3, 28, 39, 5); rect(g, 14, 7, 20, 29, 2, { fill: color, opacity: '.4' }); g.appendChild(el('circle', { cx: 24, cy: 39, r: 1.5, fill: color })); }
    else if (type === 'router') { rect(g, 7, 20, 34, 17, 5); line(g, 12, 20, 8, 7, 2); line(g, 36, 20, 40, 7, 2); [14, 20, 26].forEach(function (x) { g.appendChild(el('circle', { cx: x, cy: 29, r: 2, fill: color })); }); line(g, 31, 29, 36, 29, 2); }
    else if (type === 'switch') { rect(g, 5, 14, 38, 22, 5); rect(g, 9, 18, 30, 7, 2, { fill: color, opacity: '.4' }); [10, 17, 24, 31].forEach(function (x) { rect(g, x, 29, 4, 3, 1, { fill: color }); }); }
    else if (type === 'accesspoint') { rect(g, 13, 24, 22, 15, 5); g.appendChild(el('path', { d: 'M13 20c6-6 16-6 22 0M8 14c9-10 23-10 32 0', fill: 'none', stroke: '#fff', 'stroke-width': 3, 'stroke-linecap': 'round' })); g.appendChild(el('circle', { cx: 24, cy: 31, r: 2, fill: color })); }
    else if (type === 'camera') { rect(g, 6, 14, 27, 20, 5); g.appendChild(el('circle', { cx: 20, cy: 24, r: 7, fill: color, opacity: '.45' })); g.appendChild(el('circle', { cx: 20, cy: 24, r: 3, fill: '#fff' })); g.appendChild(el('path', { d: 'M33 19l10-5v20l-10-5z', fill: '#fff' })); }
    else if (type === 'tv') { rect(g, 6, 7, 36, 27, 5); rect(g, 10, 11, 28, 19, 2, { fill: color, opacity: '.4' }); line(g, 24, 34, 24, 39, 3); line(g, 16, 40, 32, 40, 3); }
    else if (type === 'iot') { g.appendChild(el('path', { d: 'M24 5c-10 0-16 11-10 19 3 4 5 5 5 10h10c0-5 2-6 5-10 6-8 0-19-10-19z', fill: '#fff' })); rect(g, 19, 35, 10, 5, 2); line(g, 18, 19, 22, 23, 2); line(g, 30, 19, 26, 23, 2); }
    else { g.appendChild(el('circle', { cx: 24, cy: 22, r: 16, fill: '#fff' })); var q = el('text', { x: 24, y: 30, 'text-anchor': 'middle', 'font-family': 'system-ui,sans-serif', 'font-size': 24, 'font-weight': 700, fill: color }); q.textContent = '?'; g.appendChild(q); }
  }
  function draw(parent, cx, cy, type) { var key = resolve(type), color = catalog[key].color; parent.appendChild(el('circle', { cx: cx, cy: cy, r: 32, fill: color })); parent.appendChild(el('circle', { cx: cx, cy: cy, r: 32, fill: 'none', stroke: 'rgba(0,0,0,.08)', 'stroke-width': 2 })); var g = el('g', { transform: 'translate(' + (cx - 24) + ',' + (cy - 24) + ')' }); drawInner(g, key, color); parent.appendChild(g); return key; }
  window.UnyPortDeviceIcons = { catalog: catalog, resolve: resolve, draw: draw };
}());
