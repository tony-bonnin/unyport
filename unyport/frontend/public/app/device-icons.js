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
    unknown:     { label: 'Équipement',     color: '#9DA8B7', asset: 'status/status.svg' },
    os_windows:       { label: 'Windows',          color: '#70A7DC', asset: 'os/windows.svg' },
    os_apple_macos:   { label: 'Apple macOS',      color: '#8798E0', asset: 'os/apple_macos.svg' },
    os_linux:         { label: 'Linux',            color: '#D9B95A', asset: 'os/linux_tux.svg' },
    os_android:       { label: 'Android',          color: '#76BB88', asset: 'os/android.svg' },
    os_debian:        { label: 'Debian',           color: '#DC89B2', asset: 'os/debian.svg' },
    os_ubuntu:        { label: 'Ubuntu',           color: '#EBA071', asset: 'os/ubuntu.svg' },
    os_alpine_linux:  { label: 'Alpine Linux',     color: '#75B2D2', asset: 'os/alpine_linux.svg' },
    os_fedora:        { label: 'Fedora',           color: '#70A7DC', asset: 'os/fedora.svg' },
    os_arch_linux:    { label: 'Arch Linux',       color: '#75B2D2', asset: 'os/arch_linux.svg' },
    os_linux_mint:    { label: 'Linux Mint',       color: '#76BB88', asset: 'os/linux_mint.svg' },
    os_opensuse:      { label: 'openSUSE',         color: '#62B7AD', asset: 'os/opensuse.svg' },
    os_red_hat:       { label: 'Red Hat',          color: '#E8878D', asset: 'os/red_hat.svg' },
    os_rocky_linux:   { label: 'Rocky Linux',      color: '#62B7AD', asset: 'os/rocky_linux.svg' },
    os_almalinux:     { label: 'AlmaLinux',        color: '#8798E0', asset: 'os/almalinux.svg' },
    os_centos_stream: { label: 'CentOS Stream',    color: '#A18AD6', asset: 'os/centos_stream.svg' },
    os_freebsd:       { label: 'FreeBSD',          color: '#E8878D', asset: 'os/freebsd.svg' },
    os_openbsd:       { label: 'OpenBSD',          color: '#EBA071', asset: 'os/openbsd.svg' },
    os_netbsd:        { label: 'NetBSD',           color: '#DC89B2', asset: 'os/netbsd.svg' },
    os_chromeos:      { label: 'ChromeOS',         color: '#70A7DC', asset: 'os/chromeos.svg' },
    os_ios:           { label: 'Apple iOS',        color: '#8798E0', asset: 'os/ios.svg' },
    os_raspberry_pi:  { label: 'Raspberry Pi OS',  color: '#DC89B2', asset: 'os/raspberry_pi_os.svg' },
    os_kali_linux:    { label: 'Kali Linux',       color: '#70A7DC', asset: 'os/kali_linux.svg' },
    os_nixos:         { label: 'NixOS',            color: '#75B2D2', asset: 'os/nixos.svg' },
    os_gentoo:        { label: 'Gentoo',           color: '#A18AD6', asset: 'os/gentoo.svg' },
    os_manjaro:       { label: 'Manjaro',          color: '#76BB88', asset: 'os/manjaro.svg' },
    os_void_linux:    { label: 'Void Linux',       color: '#62B7AD', asset: 'os/void_linux.svg' },
    os_elementary:    { label: 'elementary OS',    color: '#9DA8B7', asset: 'os/elementary_os.svg' },
    os_zorin:         { label: 'Zorin OS',         color: '#70A7DC', asset: 'os/zorin_os.svg' }
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
    var networkStorage = key === 'nas' || key === 'san';
    return catalog[key] && ((catalog[key].asset.indexOf('storage/') !== 0 && key !== 'storage') || networkStorage) ? key : 'unknown';
  }
  function resolveStorage(type) {
    var key = resolve(type);
    return catalog[key] && (catalog[key].asset.indexOf('storage/') === 0 || key === 'storage') ? key : 'storage';
  }
  function resolveOS(value) {
    var name = String(value || '').trim().toLowerCase().replace(/[^a-z0-9]+/g, '_');
    var patterns = [
      ['os_raspberry_pi', /(raspberry|raspbian)/], ['os_kali_linux', /(^|_)kali(_|$)/],
      ['os_alpine_linux', /(^|_)alpine(_|$)/], ['os_arch_linux', /(^|_)arch(_|$)/],
      ['os_linux_mint', /(^|_)mint(_|$)/], ['os_rocky_linux', /(^|_)rocky(_|$)/],
      ['os_centos_stream', /(^|_)centos(_|$)/], ['os_void_linux', /(^|_)void(_|$)/],
      ['os_red_hat', /(^|_)(redhat|rhel)(_|$)/], ['os_apple_macos', /(macbook|imac|macos|osx)/],
      ['os_windows', /(^|_)(windows|win10|win11)(_|$)/], ['os_android', /(^|_)android(_|$)/],
      ['os_debian', /(^|_)debian(_|$)/], ['os_ubuntu', /(^|_)ubuntu(_|$)/],
      ['os_fedora', /(^|_)fedora(_|$)/], ['os_opensuse', /(opensuse|suse)/],
      ['os_almalinux', /(^|_)alma(linux)?(_|$)/], ['os_freebsd', /(^|_)freebsd(_|$)/],
      ['os_openbsd', /(^|_)openbsd(_|$)/], ['os_netbsd', /(^|_)netbsd(_|$)/],
      ['os_chromeos', /(chromeos|chromebook)/], ['os_ios', /(^|_)(ios|iphone|ipad)(_|$)/],
      ['os_nixos', /(^|_)nixos(_|$)/], ['os_gentoo', /(^|_)gentoo(_|$)/],
      ['os_manjaro', /(^|_)manjaro(_|$)/], ['os_elementary', /(^|_)elementary(_|$)/],
      ['os_zorin', /(^|_)zorin(_|$)/], ['os_linux', /(^|_)linux(_|$)/]
    ];
    for (var i = 0; i < patterns.length; i++) if (patterns[i][1].test(name)) return patterns[i][0];
    return '';
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
  function drawOS(parent, cx, cy, value) { var key = resolveOS(value); return key ? drawKey(parent, cx, cy, key) : ''; }
  window.UnyPortDeviceIcons = { catalog: catalog, resolve: resolve, resolveNetwork: resolveNetwork, resolveStorage: resolveStorage, resolveOS: resolveOS, draw: draw, drawNetwork: drawNetwork, drawOS: drawOS };
}());
