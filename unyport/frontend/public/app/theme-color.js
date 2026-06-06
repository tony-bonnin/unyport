// Theme-color synchronizer for <meta name="theme-color">.
(function () {
  var FALLBACK_THEME_COLOR = '#3e3aab';
  var THEME_COLOR_BY_ROLE = {
    light: {
      dom0: '#3e3aab',
      domu: '#a0284a',
      container: '#b05a1a',
      alpine: '#28587c',
    },
    dark: {
      dom0: '#6864d4',
      domu: '#d45c82',
      container: '#de8d50',
      alpine: '#4d8cb0',
    },
  };

  var themeColorMeta = null;
  var syncInProgress = false;
  var fallbackTimer = null;
  var themeColorObserver = null;
  var rootTheme = null;
  var nextFrame = (window && window.requestAnimationFrame) ? window.requestAnimationFrame.bind(window) : function (cb) { setTimeout(cb, 0); };

  function normalizeRole(role) {
    return String(role || '').trim().toLowerCase();
  }

  function normalizeTheme(theme) {
    return String(theme || '').trim().toLowerCase() === 'dark' ? 'dark' : 'light';
  }

  function themeFromState(role, theme) {
    var roleKey = normalizeRole(role);
    var themeKey = normalizeTheme(theme || (rootTheme && rootTheme.getAttribute ? rootTheme.getAttribute('data-theme') : 'light'));
    var palette = THEME_COLOR_BY_ROLE[themeKey] || THEME_COLOR_BY_ROLE.light;
    if (roleKey && palette[roleKey]) return palette[roleKey];
    return palette.dom0 || FALLBACK_THEME_COLOR;
  }

  function setThemeColor(theme, role) {
    if (!themeColorMeta) {
      themeColorMeta = document.querySelector('meta[name="theme-color"]');
    }
    if (!themeColorMeta) return;

    if (!rootTheme) {
      rootTheme = document.documentElement;
    }
    if (rootTheme && rootTheme.getAttribute('data-auth-page') === 'login') {
      themeColorMeta.setAttribute('content', FALLBACK_THEME_COLOR);
      return;
    }

    var roleToUse = role;
    if (!roleToUse && rootTheme) {
      roleToUse = rootTheme.getAttribute('data-role');
    }
    if (!theme) {
      theme = rootTheme ? rootTheme.getAttribute('data-theme') : 'light';
    }

    var color = themeFromState(roleToUse, theme);
    if (window.getComputedStyle && rootTheme) {
      var brand = getComputedStyle(rootTheme).getPropertyValue('--brand');
      brand = String(brand || '').replace(/^\s+|\s+$/g, '');
      if (brand) color = brand;
    }
    themeColorMeta.setAttribute('content', color || FALLBACK_THEME_COLOR);
  }

  function syncThemeColor(theme, role) {
    if (syncInProgress) return;
    syncInProgress = true;
    setThemeColor(theme, role);
    nextFrame(function () {
      setThemeColor(theme, role);
      syncInProgress = false;
    });
  }

  function watchThemeChange() {
    if (!rootTheme || !window.MutationObserver) {
      if (fallbackTimer) {
        clearInterval(fallbackTimer);
      }
      fallbackTimer = setInterval(function () { setThemeColor(); }, 1000);
      return;
    }
    if (themeColorObserver) return;
    themeColorObserver = new MutationObserver(function () {
      syncThemeColor();
    });
    themeColorObserver.observe(rootTheme, { attributes: true, attributeFilter: ['data-theme', 'data-role'] });
  }

  function bindEvents() {
    if (document.addEventListener) {
      document.addEventListener('DOMContentLoaded', setThemeColor);
      window.addEventListener('themechange', function () { syncThemeColor(); });
      window.__unyportSyncThemeColor = syncThemeColor;
      setThemeColor();
      watchThemeChange();
      return;
    }

    if (document.attachEvent) {
      window.attachEvent('onload', function () {
        window.__unyportSyncThemeColor = function (theme, role) { syncThemeColor(theme, role); };
        setThemeColor();
        watchThemeChange();
      });
    }
  }

  function init() {
    if (rootTheme === null) {
      rootTheme = document.documentElement;
    }
    bindEvents();
  }

  init();
})();
