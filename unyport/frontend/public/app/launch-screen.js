(() => {
  const roleKey = 'unyport-host-role';
  const applyRole = (role) => {
    const normalized = String(role || '').trim().toLowerCase();
    if (!['dom0', 'domu', 'container', 'alpine'].includes(normalized)) return;
    document.documentElement.setAttribute('data-role', normalized);
    try { window.localStorage.setItem(roleKey, normalized); } catch { /* no-op */ }
  };

  try { applyRole(window.localStorage.getItem(roleKey)); } catch { /* no-op */ }

  const dismiss = () => {
    const screen = document.getElementById('launch-screen');
    if (!screen) return;
    screen.classList.add('is-ready');
    window.setTimeout(() => screen.remove(), 140);
  };

  document.addEventListener('alpine:initialized', dismiss, { once: true });
  window.addEventListener('load', () => window.setTimeout(dismiss, 1500), { once: true });
})();
