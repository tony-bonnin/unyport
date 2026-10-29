// device-icons.js | UnyPort dual-tone device icon set
(function () {
  'use strict';
  var NS = 'http://www.w3.org/2000/svg';
  var catalog = {
    desktop:     { label: 'PC',             color: '#70A7DC', asset: '01_desktop.svg' },
    laptop:      { label: 'Portable',       color: '#8798E0', asset: '02_laptop.svg' },
    server:      { label: 'Serveur',        color: '#A18AD6', asset: '03_server.svg' },
    nas:         { label: 'NAS',            color: '#BA85BC', asset: '04_storage.svg' },
    printer:     { label: 'Imprimante',     color: '#DC89B2', asset: '05_printer.svg' },
    phone:       { label: 'Téléphone',      color: '#E8878D', asset: '06_phone.svg' },
    tablet:      { label: 'Tablette',       color: '#EBA071', asset: '07_tablet.svg' },
    router:      { label: 'Routeur',        color: '#D9B95A', asset: '08_router.svg' },
    switch:      { label: 'Switch',         color: '#62B7AD', asset: '09_switch.svg' },
    accesspoint: { label: "Point d'accès", color: '#76BB88', asset: '10_wifi.svg' },
    camera:      { label: 'Caméra',         color: '#8DAC9D', asset: '11_camera.svg' },
    tv:          { label: 'Télévision',     color: '#75B2D2', asset: '12_monitor.svg' },
    iot:         { label: 'Objet connecté', color: '#ACA76D', asset: '13_lightbulb.svg' },
    unknown:     { label: 'Équipement',     color: '#9DA8B7', asset: '14_status.svg' }
  };
  var aliases = { pc: 'desktop', computer: 'desktop', workstation: 'desktop', notebook: 'laptop', mobile: 'phone', smartphone: 'phone', print: 'printer', storage: 'nas', access_point: 'accesspoint', ap: 'accesspoint', wifi: 'accesspoint', monitor: 'tv', television: 'tv', smarttv: 'tv', lightbulb: 'iot', appliance: 'iot', status: 'unknown' };
  function resolve(type) {
    var key = String(type || '').trim().toLowerCase().replace(/[ -]+/g, '_');
    key = aliases[key] || key.replace(/_/g, '');
    return catalog[key] ? key : 'unknown';
  }
  function draw(parent, cx, cy, type) {
    var key = resolve(type);
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
  window.UnyPortDeviceIcons = { catalog: catalog, resolve: resolve, draw: draw };
}());
