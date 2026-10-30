// runtime-style.js | CSP-safe dynamic layout through a same-origin stylesheet
(function () {
  'use strict';

  var nextID = 1;
  var rules = new WeakMap();
  var allowedProperties = new Set([
    'left', 'top', 'right', 'bottom', 'width', 'height', 'background',
    'overflow-y', 'touch-action', 'pointer-events',
    '--role-dom0', '--role-domu', '--role-container', '--role-alpine',
    '--cpu-pct', '--mem-pct', '--cpu-color', '--mem-color',
    '--core-w', '--disk-w', '--temp-w', '--htop-cpu', '--htop-cpu-cls',
    '--htop-mem', '--htop-cache-l', '--htop-cache-w', '--w', '--tp-w',
    '--nm-tt-x', '--nm-tt-y'
  ]);

  function runtimeSheet() {
    var link = document.getElementById('runtime-stylesheet');
    if (!link || !link.sheet) return null;
    return link.sheet;
  }

  function safeValue(value) {
    var text = String(value == null ? '' : value).trim();
    if (text === '') return '';
    if (/^-?\d+(?:\.\d+)?(?:px|%)$/.test(text)) return text;
    if (/^#[0-9a-f]{3,8}$/i.test(text)) return text;
    if (/^(?:auto|hidden|none|ok|warn|crit)$/.test(text)) return text;
    throw new Error('Rejected unsafe runtime CSS value');
  }

  function ruleFor(element) {
    var cached = rules.get(element);
    if (cached) return cached;
    var sheet = runtimeSheet();
    if (!sheet) return null;
    var id = String(nextID++);
    element.setAttribute('data-runtime-style', id);
    var index = sheet.cssRules.length;
    sheet.insertRule('[data-runtime-style="' + id + '"]{}', index);
    var rule = sheet.cssRules[index];
    rules.set(element, rule);
    return rule;
  }

  function setRuntimeStyles(element, declarations) {
    if (!element || !declarations) return;
    var rule = ruleFor(element);
    if (!rule) return;
    Object.keys(declarations).forEach(function (property) {
      if (!allowedProperties.has(property)) throw new Error('Rejected runtime CSS property');
      var value = safeValue(declarations[property]);
      if (value) rule.style.setProperty(property, value);
      else rule.style.removeProperty(property);
    });
  }

  window.setRuntimeStyles = setRuntimeStyles;
})();
