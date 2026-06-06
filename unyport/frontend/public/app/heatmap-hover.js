(() => {
  const CARD_WIDTH = 304;
  const CARD_HEIGHT = 220;
  const GUTTER = 12;

  const state = {
    hoverRule: null,
  };

  function getHoverRule() {
    if (state.hoverRule) return state.hoverRule;
    const styleSheets = Array.from(document.styleSheets || []);
    for (const sheet of styleSheets) {
      let rules;
      try {
        rules = sheet.cssRules || [];
      } catch (_) {
        continue;
      }
      for (const rule of rules) {
        if (rule && rule.selectorText === '.reboot-hover-card') {
          state.hoverRule = rule;
          return rule;
        }
      }
    }
    for (const sheet of styleSheets) {
      try {
        const index = (sheet.cssRules || []).length;
        sheet.insertRule('.reboot-hover-card {}', index);
        state.hoverRule = sheet.cssRules[index];
        return state.hoverRule;
      } catch (_) {
        continue;
      }
    }
    return null;
  }

  function levelLabel(level) {
    const value = Number(level) || 0;
    if (value >= 3) return 'High activity';
    if (value === 2) return 'Medium activity';
    if (value === 1) return 'Low activity';
    return 'No activity';
  }

  function countLabel(count) {
    const value = Number(count) || 0;
    return `${value} restart${value > 1 ? 's' : ''}`;
  }

  function positionCard(target) {
    const rule = getHoverRule();
    if (!rule || !target || !target.getBoundingClientRect) return;
    const rect = target.getBoundingClientRect();
    const vw = window.innerWidth || 0;
    const vh = window.innerHeight || 0;
    let left = rect.right + GUTTER;
    let top = rect.top + (rect.height / 2) - (CARD_HEIGHT / 2);
    if (left + CARD_WIDTH > vw - 12) left = rect.left - CARD_WIDTH - GUTTER;
    left = Math.max(12, Math.min(left, vw - CARD_WIDTH - 12));
    top = Math.max(12, Math.min(top, vh - CARD_HEIGHT - 12));
    rule.style.left = `${left}px`;
    rule.style.top = `${top}px`;
    rule.style.right = 'auto';
    rule.style.bottom = 'auto';
  }

  function showCard(target) {
    const card = document.getElementById('reboot-hover-card');
    if (!card || !target) return;
    const date = target.dataset.rebootDate || '—';
    const label = target.dataset.rebootLabel || date;
    const count = target.dataset.rebootCount || '0';
    const level = target.dataset.rebootLevel || '0';
    const today = target.dataset.rebootToday === '1';

    const dateEl = document.getElementById('reboot-hover-date');
    const countEl = document.getElementById('reboot-hover-count');
    const dayEl = document.getElementById('reboot-hover-day');
    const levelEl = document.getElementById('reboot-hover-level');
    const statusEl = document.getElementById('reboot-hover-status');
    if (dateEl) dateEl.textContent = date;
    if (countEl) countEl.textContent = countLabel(count);
    if (dayEl) dayEl.textContent = label;
    if (levelEl) levelEl.textContent = levelLabel(level);
    if (statusEl) statusEl.textContent = today ? 'Current day' : 'Archived day';

    positionCard(target);
    card.classList.add('is-visible');
    card.setAttribute('aria-hidden', 'false');
  }

  function hideCard() {
    const card = document.getElementById('reboot-hover-card');
    if (!card) return;
    card.classList.remove('is-visible');
    card.setAttribute('aria-hidden', 'true');
  }

  function bindButtons() {
    const buttons = document.querySelectorAll('[data-reboot-heatmap-cell="1"]');
    buttons.forEach((button) => {
      if (button.dataset.rebootHoverBound === '1') return;
      button.dataset.rebootHoverBound = '1';
      button.addEventListener('mouseenter', () => showCard(button));
      button.addEventListener('focus', () => showCard(button));
      button.addEventListener('mouseleave', hideCard);
      button.addEventListener('blur', hideCard);
      button.addEventListener('mousemove', () => positionCard(button));
    });
  }

  function init() {
    bindButtons();
    const observer = new MutationObserver(() => bindButtons());
    observer.observe(document.body, { childList: true, subtree: true });
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
