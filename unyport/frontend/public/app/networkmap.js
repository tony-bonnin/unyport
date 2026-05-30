// networkmap.js — TRINITY · Network Map
// Layout : CSS Grid (nm-grid-N, nm-col-N) — zéro marginLeft/marginRight
// Positions X calculées depuis n° colonne (trivial, pas de DOM measurement)
// Seule la hauteur du manche est mesurée via getBoundingClientRect

var _NS = 'http://www.w3.org/2000/svg';
var NM_CW = 130, NM_GAP = 28, NM_STEP = 158, NM_MAXCOLS = 6;

function _fmtBpsMap(b) { if (!b || b <= 0) return '0 B/s'; if (b >= 1e6) return (b / 1e6).toFixed(1) + ' MB/s'; if (b >= 1e3) return (b / 1e3).toFixed(0) + ' KB/s'; return Math.round(b) + ' B/s'; }
function _fmtBytes(b) { if (!b || b <= 0) return '0 B'; if (b >= 1e9) return (b / 1e9).toFixed(1) + ' GB'; if (b >= 1e6) return (b / 1e6).toFixed(1) + ' MB'; if (b >= 1e3) return (b / 1e3).toFixed(0) + ' KB'; return b + ' B'; }
function _el(t, a) { var e = document.createElementNS(_NS, t); if (a) Object.keys(a).forEach(function (k) { e.setAttribute(k, String(a[k])); }); return e; }
function _div(c) { var d = document.createElement('div'); d.className = c; return d; }
function _mkSvg(fn) { var s = document.createElementNS(_NS, 'svg'); s.setAttribute('width', '64'); s.setAttribute('height', '64'); s.setAttribute('viewBox', '0 0 64 64'); fn(s, 32, 32); return s; }
function _truncate(v, max) { v = v == null ? '' : String(v); return v.length > max ? v.slice(0, max - 1) + '…' : v; }

// Icônes (identiques)
function _icoHost(p, cx, cy, role) { var R = 32; if (role === 'DomU') { p.appendChild(_el('circle', { cx: cx, cy: cy, r: R, fill: '#e53935' })); p.appendChild(_el('circle', { cx: cx, cy: cy, r: R, fill: 'none', stroke: 'rgba(0,0,0,.08)', 'stroke-width': 2 })); _icoServerInner(p, cx, cy, '#fff', 'rgba(255,255,255,0.3)'); } else if (role === 'Container') { p.appendChild(_el('circle', { cx: cx, cy: cy, r: R, fill: '#ff7043' })); p.appendChild(_el('circle', { cx: cx, cy: cy, r: R, fill: 'none', stroke: 'rgba(0,0,0,.08)', 'stroke-width': 2 })); _icoCubeInner(p, cx, cy, '#fff'); } else { p.appendChild(_el('circle', { cx: cx, cy: cy, r: R, fill: '#26c6c6' })); p.appendChild(_el('circle', { cx: cx, cy: cy, r: R, fill: 'none', stroke: 'rgba(0,0,0,.08)', 'stroke-width': 2 })); var cg = _el('g', { transform: 'translate(' + (cx - 18) + ',' + (cy - 12) + ')' }); cg.appendChild(_el('ellipse', { cx: 18, cy: 18, rx: 14, ry: 10, fill: '#fff' })); cg.appendChild(_el('circle', { cx: 10, cy: 16, r: 8, fill: '#fff' })); cg.appendChild(_el('circle', { cx: 22, cy: 13, r: 10, fill: '#fff' })); cg.appendChild(_el('circle', { cx: 30, cy: 17, r: 7, fill: '#fff' })); cg.appendChild(_el('line', { x1: 12, y1: 26, x2: 10, y2: 32, stroke: '#26c6c6', 'stroke-width': 2, 'stroke-linecap': 'round' })); cg.appendChild(_el('line', { x1: 18, y1: 28, x2: 16, y2: 34, stroke: '#26c6c6', 'stroke-width': 2, 'stroke-linecap': 'round' })); cg.appendChild(_el('line', { x1: 24, y1: 26, x2: 22, y2: 32, stroke: '#26c6c6', 'stroke-width': 2, 'stroke-linecap': 'round' })); p.appendChild(cg); } }
function _icoIfaceUp(p, cx, cy) { var R = 32; p.appendChild(_el('circle', { cx: cx, cy: cy, r: R, fill: '#26c6c6' })); p.appendChild(_el('circle', { cx: cx, cy: cy, r: R, fill: 'none', stroke: 'rgba(0,0,0,.08)', 'stroke-width': 2 })); var mg = _el('g', { transform: 'translate(' + (cx - 15) + ',' + (cy - 14) + ')' }); mg.appendChild(_el('rect', { x: 0, y: 0, width: 30, height: 20, rx: 3, fill: '#fff' })); mg.appendChild(_el('rect', { x: 2, y: 2, width: 26, height: 14, rx: 2, fill: '#26c6c6', opacity: '0.4' })); mg.appendChild(_el('rect', { x: 11, y: 20, width: 8, height: 5, fill: '#fff' })); mg.appendChild(_el('rect', { x: 7, y: 24, width: 16, height: 3, rx: 1, fill: '#fff' })); mg.appendChild(_el('rect', { x: 6, y: 14, width: 3, height: 4, rx: 1, fill: '#fff', opacity: '0.9' })); mg.appendChild(_el('rect', { x: 11, y: 11, width: 3, height: 7, rx: 1, fill: '#fff', opacity: '0.9' })); mg.appendChild(_el('rect', { x: 16, y: 8, width: 3, height: 10, rx: 1, fill: '#fff', opacity: '0.9' })); mg.appendChild(_el('rect', { x: 21, y: 6, width: 3, height: 12, rx: 1, fill: '#fff', opacity: '0.9' })); p.appendChild(mg); }
function _icoIfaceDn(p, cx, cy) { var R = 32; p.appendChild(_el('circle', { cx: cx, cy: cy, r: R, fill: '#ffa726' })); p.appendChild(_el('circle', { cx: cx, cy: cy, r: R, fill: 'none', stroke: 'rgba(0,0,0,.08)', 'stroke-width': 2 })); var mg = _el('g', { transform: 'translate(' + (cx - 15) + ',' + (cy - 13) + ')' }); mg.appendChild(_el('rect', { x: 0, y: 0, width: 30, height: 20, rx: 3, fill: '#fff' })); mg.appendChild(_el('rect', { x: 2, y: 2, width: 26, height: 14, rx: 2, fill: 'rgba(255,167,38,0.3)' })); mg.appendChild(_el('line', { x1: 9, y1: 6, x2: 21, y2: 18, stroke: '#fff', 'stroke-width': 2.5, 'stroke-linecap': 'round' })); mg.appendChild(_el('line', { x1: 21, y1: 6, x2: 9, y2: 18, stroke: '#fff', 'stroke-width': 2.5, 'stroke-linecap': 'round' })); mg.appendChild(_el('rect', { x: 11, y: 20, width: 8, height: 5, fill: '#fff' })); mg.appendChild(_el('rect', { x: 7, y: 24, width: 16, height: 3, rx: 1, fill: '#fff' })); p.appendChild(mg); }
function _icoNbAlive(p, cx, cy) { var R = 32; p.appendChild(_el('circle', { cx: cx, cy: cy, r: R, fill: '#42a5f5' })); p.appendChild(_el('circle', { cx: cx, cy: cy, r: R, fill: 'none', stroke: 'rgba(0,0,0,.08)', 'stroke-width': 2 })); var dg = _el('g', { transform: 'translate(' + (cx - 14) + ',' + (cy - 14) + ')' }); dg.appendChild(_el('rect', { x: 0, y: 0, width: 28, height: 18, rx: 2, fill: '#fff' })); dg.appendChild(_el('rect', { x: 1, y: 1, width: 26, height: 14, rx: 1, fill: '#42a5f5', opacity: '0.35' })); dg.appendChild(_el('polyline', { points: '3,12 7,8 11,10 15,5 19,7 23,4', fill: 'none', stroke: '#fff', 'stroke-width': 1.5, 'stroke-linejoin': 'round', 'stroke-linecap': 'round' })); dg.appendChild(_el('rect', { x: 10, y: 18, width: 8, height: 4, fill: '#fff' })); dg.appendChild(_el('rect', { x: 6, y: 21, width: 16, height: 3, rx: 1, fill: '#fff' })); p.appendChild(dg); }
function _icoNbStale(p, cx, cy) { var R = 32; p.appendChild(_el('circle', { cx: cx, cy: cy, r: R, fill: '#8741ff' })); p.appendChild(_el('circle', { cx: cx, cy: cy, r: R, fill: 'none', stroke: 'rgba(0,0,0,.08)', 'stroke-width': 2 })); var dg = _el('g', { transform: 'translate(' + (cx - 14) + ',' + (cy - 14) + ')' }); dg.appendChild(_el('rect', { x: 0, y: 0, width: 28, height: 18, rx: 2, fill: 'rgba(255,255,255,0.9)' })); dg.appendChild(_el('rect', { x: 1, y: 1, width: 26, height: 14, rx: 1, fill: 'rgba(135,65,255,0.25)' })); dg.appendChild(_el('polyline', { points: '3,12 7,8 11,10 15,5 19,7 23,4', fill: 'none', stroke: 'rgba(255,255,255,0.8)', 'stroke-width': 1.5, 'stroke-dasharray': '3 2', 'stroke-linejoin': 'round', 'stroke-linecap': 'round' })); dg.appendChild(_el('rect', { x: 10, y: 18, width: 8, height: 4, fill: 'rgba(255,255,255,0.9)' })); dg.appendChild(_el('rect', { x: 6, y: 21, width: 16, height: 3, rx: 1, fill: 'rgba(255,255,255,0.9)' })); p.appendChild(dg); }
function _icoNbUnk(p, cx, cy) { var R = 32; p.appendChild(_el('circle', { cx: cx, cy: cy, r: R, fill: '#90a4ae' })); p.appendChild(_el('circle', { cx: cx, cy: cy, r: R, fill: 'none', stroke: 'rgba(0,0,0,.08)', 'stroke-width': 2 })); var qt = _el('text', { x: cx, y: cy + 8, 'text-anchor': 'middle', 'font-family': 'Inter,system-ui,sans-serif', 'font-size': '28', 'font-weight': '700', fill: '#fff', opacity: '0.9' }); qt.textContent = '?'; p.appendChild(qt); }
function _icoServerInner(p, cx, cy, fill, fa) { var sg = _el('g', { transform: 'translate(' + (cx - 14) + ',' + (cy - 16) + ')' }); sg.appendChild(_el('rect', { x: 2, y: 8, width: 24, height: 18, rx: 2, fill: fill })); sg.appendChild(_el('ellipse', { cx: 14, cy: 8, rx: 12, ry: 5, fill: fill })); sg.appendChild(_el('ellipse', { cx: 14, cy: 26, rx: 12, ry: 5, fill: fa || 'rgba(255,255,255,0.3)' })); sg.appendChild(_el('ellipse', { cx: 14, cy: 14, rx: 12, ry: 3, fill: 'none', stroke: fa || 'rgba(255,255,255,0.3)', 'stroke-width': 1 })); sg.appendChild(_el('ellipse', { cx: 14, cy: 20, rx: 12, ry: 3, fill: 'none', stroke: fa || 'rgba(255,255,255,0.3)', 'stroke-width': 1 })); p.appendChild(sg); }
function _icoCubeInner(p, cx, cy, fill) { var cg = _el('g', { transform: 'translate(' + (cx - 16) + ',' + (cy - 16) + ')' }); cg.appendChild(_el('rect', { x: 4, y: 10, width: 20, height: 16, rx: 2, fill: fill })); cg.appendChild(_el('polygon', { points: '4,10 14,4 28,4 18,10', fill: fill, opacity: '0.7' })); cg.appendChild(_el('polygon', { points: '24,10 28,4 28,20 24,26', fill: fill, opacity: '0.5' })); p.appendChild(cg); }

// Card HTML
function _mkCard(drawFn, label, sub, tip) {
  var card = _div('nm-card'); if (tip) card.title = tip;
  var icon = _div('nm-card-icon'); icon.appendChild(_mkSvg(drawFn)); card.appendChild(icon);
  var lbl = _div('nm-card-label'); lbl.textContent = _truncate(label, 16); card.appendChild(lbl);
  if (sub) { var s = _div('nm-card-sub'); s.textContent = _truncate(sub, 18); card.appendChild(s); }
  return card;
}
function _nbCard(nb) {
  var ip = nb.ip || '?', mac = nb.mac || '—', state = nb.state || '—', iface = nb.iface || '—';
  return _mkCard(function (s, cx, cy) { if (state === 'reachable') _icoNbAlive(s, cx, cy); else if (state === 'stale') _icoNbStale(s, cx, cy); else _icoNbUnk(s, cx, cy); },
    ip, nb.mac ? mac.substring(0, 17) : state, [ip, 'MAC: ' + mac, 'État: ' + state, 'Interface: ' + iface].join('\n'));
}

// Position X du centre de la colonne col (1-indexé)
function _cx(col) { return (col - 1) * NM_STEP + NM_CW / 2; }

// Calcul du nombre de colonnes selon la largeur du container
function _calcCols(el) {
  var w = el ? (el.clientWidth || el.offsetWidth || 0) : 0;
  if (w < 10 && el && el.parentElement) { var p = el.parentElement; while (p && (p.clientWidth || p.offsetWidth || 0) < 10) p = p.parentElement; if (p) w = p.clientWidth || p.offsetWidth || 0; }
  if (w < 10) w = window.innerWidth || 800;
  return Math.max(1, Math.min(NM_MAXCOLS, Math.floor((w - 64 + NM_GAP) / NM_STEP)));
}

function _neighborsByIface(ifaces, neighbors) {
  var byName = Object.create(null);
  ifaces.forEach(function (iface) { byName[iface.name] = []; });
  neighbors.forEach(function (n) {
    if (n && byName[n.iface]) byName[n.iface].push(n);
  });
  return byName;
}

function _firstChildRect(connect) {
  var row = connect.nextElementSibling;
  return row && row.children[0] ? row.children[0].getBoundingClientRect() : null;
}

// Ajouter un connecteur dans nm-connect (positions en px depuis _cx)
function _conn(parent, cls, l, t, w, h) {
  var d = _div(cls);
  if (l !== null) d.style.left = l + 'px';
  if (t !== null) d.style.top = t + 'px';
  if (w !== null) d.style.width = w + 'px';
  if (h !== null) d.style.height = h + 'px';
  parent.appendChild(d);
  return d;
}

// ── renderNetworkMap ──
function renderNetworkMap(netMap, hostRole, hostRuntime, hostName, hostIP, containerId) {
  var el = document.getElementById(containerId || 'netmap-container');
  if (!el) return;
  var ifaces = netMap && Array.isArray(netMap.interfaces) ? netMap.interfaces : [];
  var neighbors = netMap && Array.isArray(netMap.neighbors) ? netMap.neighbors : [];
  if (ifaces.length === 0) { el.innerHTML = '<p class="nm-empty">Aucune interface réseau…</p>'; return; }

  var renderId = (el._nmRenderId || 0) + 1;
  el._nmRenderId = renderId;
  var nbPerIface = _neighborsByIface(ifaces, neighbors);
  var COLS = _calcCols(el);
  var role = hostRole || 'Alpine';
  var root = _div('nm-level');

  // Host
  var hostRow = _div('nm-row');
  var hostCard = _mkCard(function (s, cx, cy) { _icoHost(s, cx, cy, role); }, hostName || 'host', hostIP || '',
    [hostName || 'host', 'Rôle: ' + role, 'IP: ' + (hostIP || '—')].join('\n'));
  hostRow.appendChild(hostCard);
  root.appendChild(hostRow);

  var pending = [];

  for (var start = 0, ci = 0; start < ifaces.length; start += COLS, ci++) {
    var chunkLen = Math.min(COLS, ifaces.length - start);
    var connect = _div('nm-connect nm-connect-' + COLS);
    root.appendChild(connect);

    var row = _div('nm-grid nm-grid-' + COLS);
    var ifCards = [];
    for (var pos = 0; pos < chunkLen; pos++) {
      var idx = start + pos;
      var iface = ifaces[idx], up = iface.up !== false;
      var col = pos + 1;
      var card = _mkCard(function (s, cx, cy) { up ? _icoIfaceUp(s, cx, cy) : _icoIfaceDn(s, cx, cy); },
        iface.name, iface.ip || '—',
        [iface.name, 'IP: ' + (iface.ip || '—'), 'État: ' + (up ? 'UP' : 'DOWN'), '↓ ' + _fmtBpsMap(iface.rx_bps || 0), '↑ ' + _fmtBpsMap(iface.tx_bps || 0)].join('\n'));
      card.classList.add('nm-col-' + col);
      card._col = col; card._ifaceName = iface.name;
      row.appendChild(card);
      ifCards.push(card);
    }
    root.appendChild(row);

    if (ci === 0) {
      pending.push({ type: 'host-rake', connect: connect, childCols: ifCards.map(function (c) { return c._col; }), COLS: COLS });
    } else {
      pending.push({ type: 'host-rake-next', connect: connect, hostCard: hostCard, childCols: ifCards.map(function (c) { return c._col; }), COLS: COLS });
    }

    ifCards.forEach(function (ifCard) {
      var nbs = nbPerIface[ifCard._ifaceName];
      if (!nbs || nbs.length === 0) return;
      var parentCol = ifCard._col;
      var isLeft = parentCol <= Math.ceil(COLS / 2);

      for (var j = 0; j < nbs.length; j += COLS) {
        var nbChunk = nbs.slice(j, j + COLS);
        var n = nbChunk.length;
        var startCol = n === 1 ? parentCol : (isLeft ? parentCol : Math.max(1, parentCol - n + 1));

        var nbConnect = _div('nm-connect nm-connect-' + COLS);
        root.appendChild(nbConnect);

        var nbRow = _div('nm-grid nm-grid-' + COLS);
        var childCols = [];
        nbChunk.forEach(function (nb, ni) {
          var col = startCol + ni;
          childCols.push(col);
          var card = _nbCard(nb);
          card.classList.add('nm-col-' + col);
          nbRow.appendChild(card);
        });
        root.appendChild(nbRow);

        pending.push({ type: 'child-rake', connect: nbConnect, parentCard: ifCard, parentCol: parentCol, childCols: childCols, COLS: COLS });
      }
    });
  }

  el.innerHTML = ''; el.appendChild(root);

  requestAnimationFrame(function () {
    if (el._nmRenderId !== renderId || !root.isConnected) return;

    pending.forEach(function (p) {
      var connR = p.connect.getBoundingClientRect();

      if (p.type === 'host-rake') {
        var BAR = 20, DROP = 15, ARR = 35, H = 42;
        p.connect.style.height = H + 'px';
        var mc = (_cx(p.childCols[0]) + _cx(p.childCols[p.childCols.length - 1])) / 2;
        _conn(p.connect, 'nm-manche', mc - 1, 0, null, BAR);
        if (p.childCols.length > 1) {
          var fc = _cx(p.childCols[0]), lc = _cx(p.childCols[p.childCols.length - 1]);
          _conn(p.connect, 'nm-bar', fc, BAR, lc - fc, null);
        }
        p.childCols.forEach(function (col) {
          _conn(p.connect, 'nm-drop', _cx(col) - 1, BAR, null, DROP);
          _conn(p.connect, 'nm-arr-down', _cx(col) - 5, ARR, null, null);
        });

      } else if (p.type === 'host-rake-next') {
        var pRH = p.hostCard.getBoundingClientRect();
        var firstCHR = _firstChildRect(p.connect);
        var connHH = firstCHR ? Math.max(firstCHR.top - connR.top, 42) : 42;
        p.connect.style.height = connHH + 'px';
        var mancheTopH = pRH.bottom - connR.top;
        var mancheHH = Math.max(connHH - mancheTopH - 22, 4);
        var barTopH = mancheTopH + mancheHH;
        var mcH = (_cx(p.childCols[0]) + _cx(p.childCols[p.childCols.length - 1])) / 2;
        _conn(p.connect, 'nm-manche', mcH - 1, mancheTopH, null, mancheHH);
        var fcH = _cx(p.childCols[0]), lcH = _cx(p.childCols[p.childCols.length - 1]);
        if (p.childCols.length > 1) _conn(p.connect, 'nm-bar', fcH, barTopH, lcH - fcH, null);
        p.childCols.forEach(function (col) {
          _conn(p.connect, 'nm-drop', _cx(col) - 1, barTopH, null, connHH - barTopH);
          _conn(p.connect, 'nm-arr-down', _cx(col) - 5, connHH - 7, null, null);
        });

      } else if (p.type === 'child-rake') {
        var pR = p.parentCard.getBoundingClientRect();
        var firstChildR = _firstChildRect(p.connect);
        var connH = firstChildR ? Math.max(firstChildR.top - connR.top, 42) : 42;
        p.connect.style.height = connH + 'px';
        var mancheTop = pR.bottom - connR.top;

        // Classe spéciale si manche traversant (parent plusieurs rangées au-dessus)
        var mancheClass = mancheTop < -10 ? 'nm-manche nm-manche-thru' : 'nm-manche';

        if (p.childCols.length === 1) {
          // Enfant seul : manche complet du parent jusqu'à la card + flèche bidi
          var mancheFullH = connH - mancheTop - 7;
          _conn(p.connect, mancheClass, _cx(p.parentCol) - 1, mancheTop, null, mancheFullH);
          _conn(p.connect, 'nm-arr-up', _cx(p.parentCol) - 5, mancheTop, null, null);
          _conn(p.connect, 'nm-arr-down', _cx(p.childCols[0]) - 5, connH - 7, null, null);
        } else {
          // Plusieurs enfants : manche + barre + descentes
          var mancheH = Math.max(connH - mancheTop - 22, 4);
          var barTop = mancheTop + mancheH;
          _conn(p.connect, mancheClass, _cx(p.parentCol) - 1, mancheTop, null, mancheH);
          _conn(p.connect, 'nm-arr-up', _cx(p.parentCol) - 5, mancheTop, null, null);
          var fc = _cx(p.childCols[0]), lc = _cx(p.childCols[p.childCols.length - 1]);
          _conn(p.connect, 'nm-bar', fc, barTop, lc - fc, null);
          p.childCols.forEach(function (col) {
            _conn(p.connect, 'nm-drop', _cx(col) - 1, barTop, null, connH - barTop);
            _conn(p.connect, 'nm-arr-down', _cx(col) - 5, connH - 7, null, null);
          });
        }
      }
    });
  });
}