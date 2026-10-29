// device-icons.js | UnyPort dual-tone device icon set
(function () {
  'use strict';
  var NS = 'http://www.w3.org/2000/svg';
  var catalog = {
    desktop:     { label: 'PC',             color: '#70A7DC', asset: 'devices/desktop.svg' },
    laptop:      { label: 'Portable',       color: '#8798E0', asset: 'devices/laptop.svg' },
    server:      { label: 'Serveur',        color: '#A18AD6', asset: 'devices/server.svg' },
    storage:     { label: 'Stockage',       color: '#BA85BC', asset: 'devices/storage.svg' },
    printer:     { label: 'Imprimante',     color: '#DC89B2', asset: 'devices/printer.svg' },
    phone:       { label: 'Téléphone',      color: '#E8878D', asset: 'devices/phone.svg' },
    tablet:      { label: 'Tablette',       color: '#EBA071', asset: 'devices/tablet.svg' },
    router:      { label: 'Routeur',        color: '#D9B95A', asset: 'network/router.svg' },
    switch:      { label: 'Switch',         color: '#62B7AD', asset: 'network/switch.svg' },
    accesspoint: { label: "Point d'accès", color: '#76BB88', asset: 'network/wifi.svg' },
    camera:      { label: 'Caméra',         color: '#8DAC9D', asset: 'devices/camera.svg' },
    tv:          { label: 'Télévision',     color: '#75B2D2', asset: 'devices/monitor.svg' },
    iot:         { label: 'Objet connecté', color: '#ACA76D', asset: 'smart-home/lightbulb.svg' },
    unknown:     { label: 'Équipement',     color: '#9DA8B7', asset: 'status/status.svg' }
  };
  (window.UnyPortExtendedDeviceTypes || []).forEach(function (entry) {
    catalog[entry.type] = {
      label: entry.type.replace(/_/g, ' ').replace(/\b\w/g, function (letter) { return letter.toUpperCase(); }),
      color: '#9DA8B7',
      asset: entry.asset
    };
  });
  var aliases = { pc: 'desktop', computer: 'desktop', workstation: 'desktop', notebook: 'laptop', mobile: 'phone', smartphone: 'phone', print: 'printer', storage: 'nas', access_point: 'accesspoint', ap: 'accesspoint', wifi: 'accesspoint', monitor: 'tv', television: 'tv', smarttv: 'tv', lightbulb: 'iot', appliance: 'iot', status: 'unknown' };
  function resolve(type) {
    var key = String(type || '').trim().toLowerCase().replace(/[ -]+/g, '_');
    if (catalog[key]) return key;
    key = aliases[key] || key.replace(/_/g, '');
    return catalog[key] ? key : 'unknown';
  }
  function resolveNetwork(type) {
    var key = resolve(type);
    return catalog[key] && catalog[key].asset.indexOf('storage/') !== 0 && key !== 'storage' ? key : 'unknown';
  }
  function resolveStorage(type) {
    var key = resolve(type);
    return catalog[key] && (catalog[key].asset.indexOf('storage/') === 0 || key === 'storage') ? key : 'storage';
  }
  function drawKey(parent, cx, cy, key) {
    var icon = document.createElementNS(NS, 'image');
    icon.setAttribute('x', String(cx - 32));
    icon.setAttribute('y', String(cy - 32));
    icon.setAttribute('width', '64');
    icon.setAttribute('height', '64');
    icon.setAttribute('href', '/media/icons/' + catalog[key].asset);
    icon.setAttribute('preserveAspectRatio', 'xMidYMid meet');
    parent.appendChild(icon);
    return key;
  }
  function draw(parent, cx, cy, type) { return drawKey(parent, cx, cy, resolve(type)); }
  function drawNetwork(parent, cx, cy, type) { return drawKey(parent, cx, cy, resolveNetwork(type)); }
  window.UnyPortDeviceIcons = { catalog: catalog, resolve: resolve, resolveNetwork: resolveNetwork, resolveStorage: resolveStorage, draw: draw, drawNetwork: drawNetwork };
}());
