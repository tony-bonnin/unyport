(function () {
  var logoutInFlight = false;
  var LOGOUT_SELECTOR = '[data-unyport-logout]';

  function isLogoutTrigger(node) {
    var el = node;
    while (el && el !== document) {
      if (el && el.nodeType === 1 && el.getAttribute) {
        if (el.getAttribute('data-unyport-logout') !== null) {
          return true;
        }
      }
      el = el.parentNode;
      if (!el || el === document.documentElement) {
        break;
      }
    }
    return false;
  }

  function getTriggerElement(node) {
    var el = node;
    while (el && el !== document) {
      if (el && el.nodeType === 1 && el.matches && el.matches(LOGOUT_SELECTOR)) {
        return el;
      }
      el = el.parentNode;
    }
    return null;
  }

  async function requestLogout() {
    if (typeof logout === 'function') {
      await logout();
      return;
    }

    localStorage.setItem('_logged_out', '1');
    await fetch('/api/logout', {
      method: 'POST',
      credentials: 'include',
      cache: 'no-store',
    });
  }

  async function handleLogoutClick(event) {
    if (!event || (event.type !== 'click' && event.type !== 'touchend')) return;
    var trigger = getTriggerElement(event.target);
    if (!trigger && !isLogoutTrigger(event.target)) return;
    if (logoutInFlight) return;

    logoutInFlight = true;
    if (typeof event.preventDefault === 'function') {
      event.preventDefault();
    }

    try {
      await requestLogout();
    } catch (e) {
      console.warn('logout request failed:', e && e.message ? e.message : e);
    } finally {
      window.location.replace('/');
    }
  }

  function bindTrigger(trigger) {
    if (!trigger || trigger.__unyportLogoutBound) return;
    trigger.__unyportLogoutBound = true;
    trigger.addEventListener('click', handleLogoutClick, true);
    trigger.addEventListener('touchend', handleLogoutClick, true);
  }

  function bindAllTriggers() {
    var triggers = document.querySelectorAll(LOGOUT_SELECTOR);
    for (var i = 0; i < triggers.length; i += 1) {
      bindTrigger(triggers[i]);
    }
  }

  if (!document.querySelectorAll || !document.addEventListener) return;

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', bindAllTriggers, false);
  } else {
    bindAllTriggers();
  }

  if (window.MutationObserver) {
    new MutationObserver(bindAllTriggers).observe(document.documentElement, {
      childList: true,
      subtree: true,
    });
  }

  document.addEventListener('click', handleLogoutClick, true);
  document.addEventListener('touchend', handleLogoutClick, true);
})();
