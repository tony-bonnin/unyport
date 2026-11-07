(() => {
  const dismiss = () => {
    const screen = document.getElementById('launch-screen');
    if (!screen) return;
    screen.classList.add('is-ready');
    window.setTimeout(() => screen.remove(), 140);
  };

  document.addEventListener('alpine:initialized', dismiss, { once: true });
  window.addEventListener('load', () => window.setTimeout(dismiss, 1500), { once: true });
})();
