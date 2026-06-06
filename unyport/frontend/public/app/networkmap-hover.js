(() => {
  const HOVER_WIDTH = 312;
  const HOVER_HEIGHT = 216;
  const GUTTER = 12;

  const state = {
    hoverRule: null,
    hoverCard: null,
    modalOverlay: null,
    hoverIcon: null,
    modalTitle: null,
    modalIcon: null,
    modalSubtitle: null,
    modalBadge: null,
    modalSummary: null,
    modalRows: null,
  };

  function getHoverRule() {
    if (state.hoverRule) return state.hoverRule;
    const sheets = Array.from(document.styleSheets || []);
    for (const sheet of sheets) {
      let rules;
      try {
        rules = sheet.cssRules || [];
      } catch (_) {
        continue;
      }
      for (const rule of rules) {
        if (rule && rule.selectorText === '.nm-hover-card') {
          state.hoverRule = rule;
          return rule;
        }
      }
    }
    for (const sheet of sheets) {
      try {
        const index = (sheet.cssRules || []).length;
        sheet.insertRule('.nm-hover-card {}', index);
        state.hoverRule = sheet.cssRules[index];
        return state.hoverRule;
      } catch (_) {
        continue;
      }
    }
    return null;
  }

  function parsePayload(node) {
    if (!node || !node.dataset || !node.dataset.nmPayload) return null;
    try {
      return JSON.parse(node.dataset.nmPayload);
    } catch (_) {
      return null;
    }
  }

  function badgeToneClass(payload) {
    const tone = payload && payload.badgeTone ? payload.badgeTone : 'muted';
    return `is-${tone}`;
  }

  function rowsHtml(rows) {
    return (rows || []).map((row) => (
      `<div class="nm-detail-row"><span>${escapeHtml(row.key || '')}</span><strong>${escapeHtml(row.value || '—')}</strong></div>`
    )).join('');
  }

  function escapeHtml(value) {
    return String(value == null ? '' : value)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  }

  function ensureHoverCard() {
    if (state.hoverCard && document.body.contains(state.hoverCard)) return state.hoverCard;
    const card = document.createElement('aside');
    card.id = 'nm-hover-card';
    card.className = 'nm-hover-card';
    card.setAttribute('aria-hidden', 'true');
    card.innerHTML = `
      <div class="nm-hover-card-head">
        <div class="nm-hover-title-block">
          <div class="nm-hover-titleline">
            <span id="nm-hover-icon" class="nm-detail-icon" aria-hidden="true"></span>
            <strong id="nm-hover-title">Network node</strong>
          </div>
          <span id="nm-hover-subtitle">—</span>
        </div>
        <span id="nm-hover-badge" class="nm-detail-badge is-muted">Info</span>
      </div>
      <hr class="nm-hover-sep">
      <p id="nm-hover-summary" class="nm-hover-summary">Network details</p>
      <div id="nm-hover-rows" class="nm-detail-grid"></div>
    `;
    document.body.appendChild(card);
    state.hoverCard = card;
    state.hoverIcon = card.querySelector('#nm-hover-icon');
    return card;
  }

  function ensureModal() {
    if (state.modalOverlay && document.body.contains(state.modalOverlay)) return state.modalOverlay;
    const overlay = document.createElement('div');
    overlay.className = 'nm-modal-overlay';
    overlay.setAttribute('aria-hidden', 'true');
    overlay.innerHTML = `
      <div class="nm-modal" role="dialog" aria-modal="true" aria-labelledby="nm-modal-title">
        <div class="nm-modal-header">
          <div class="nm-modal-header-copy">
            <div class="nm-modal-titleline">
              <span id="nm-modal-icon" class="nm-detail-icon is-modal" aria-hidden="true"></span>
              <h2 id="nm-modal-title">Network node</h2>
            </div>
            <span id="nm-modal-subtitle" class="modal-email">—</span>
          </div>
          <div class="nm-modal-header-meta">
            <span id="nm-modal-badge" class="nm-detail-badge is-muted">Info</span>
            <button type="button" class="modal-close" aria-label="Close network details">
              <i class="fa-solid fa-xmark"></i>
            </button>
          </div>
        </div>
        <div class="modal-body">
          <p id="nm-modal-summary" class="nm-modal-summary">Network details</p>
          <div id="nm-modal-rows" class="nm-detail-grid nm-detail-grid-modal"></div>
        </div>
      </div>
    `;
    document.body.appendChild(overlay);
    overlay.addEventListener('click', (event) => {
      if (event.target === overlay || event.target.closest('.modal-close')) hideModal();
    });
    state.modalOverlay = overlay;
    state.modalTitle = overlay.querySelector('#nm-modal-title');
    state.modalIcon = overlay.querySelector('#nm-modal-icon');
    state.modalSubtitle = overlay.querySelector('#nm-modal-subtitle');
    state.modalBadge = overlay.querySelector('#nm-modal-badge');
    state.modalSummary = overlay.querySelector('#nm-modal-summary');
    state.modalRows = overlay.querySelector('#nm-modal-rows');
    return overlay;
  }

  function positionHoverCard(target) {
    const rule = getHoverRule();
    if (!rule || !target || !target.getBoundingClientRect) return;
    const rect = target.getBoundingClientRect();
    const vw = window.innerWidth || 0;
    const vh = window.innerHeight || 0;
    let left = rect.right + GUTTER;
    let top = rect.top + (rect.height / 2) - (HOVER_HEIGHT / 2);
    if (left + HOVER_WIDTH > vw - 12) left = rect.left - HOVER_WIDTH - GUTTER;
    left = Math.max(12, Math.min(left, vw - HOVER_WIDTH - 12));
    top = Math.max(12, Math.min(top, vh - HOVER_HEIGHT - 12));
    rule.style.left = `${left}px`;
    rule.style.top = `${top}px`;
    rule.style.right = 'auto';
    rule.style.bottom = 'auto';
  }

  function renderNodeIcon(target, mount) {
    if (!mount) return;
    mount.innerHTML = '';
    if (!target) return;
    const svg = target.querySelector('svg');
    if (!svg) return;
    const clone = svg.cloneNode(true);
    clone.setAttribute('width', '20');
    clone.setAttribute('height', '20');
    clone.setAttribute('viewBox', '0 0 64 64');
    clone.classList.add('nm-detail-icon-svg');
    mount.appendChild(clone);
  }

  function showHover(target) {
    const payload = parsePayload(target);
    const card = ensureHoverCard();
    if (!payload || !card) return;
    const title = card.querySelector('#nm-hover-title');
    const subtitle = card.querySelector('#nm-hover-subtitle');
    const badge = card.querySelector('#nm-hover-badge');
    const summary = card.querySelector('#nm-hover-summary');
    const rows = card.querySelector('#nm-hover-rows');
    renderNodeIcon(target, state.hoverIcon);
    if (title) title.textContent = payload.title || payload.label || 'Network node';
    if (subtitle) subtitle.textContent = payload.subtitle || '—';
    if (badge) {
      badge.textContent = payload.badge || payload.kind || 'Info';
      badge.className = `nm-detail-badge ${badgeToneClass(payload)}`;
    }
    if (summary) summary.textContent = payload.summary || 'Network details';
    if (rows) rows.innerHTML = rowsHtml(payload.rows);
    positionHoverCard(target);
    card.classList.add('is-visible');
    card.setAttribute('aria-hidden', 'false');
  }

  function hideHover() {
    const card = ensureHoverCard();
    if (!card) return;
    card.classList.remove('is-visible');
    card.setAttribute('aria-hidden', 'true');
  }

  function showModal(target) {
    const payload = parsePayload(target);
    const overlay = ensureModal();
    if (!payload || !overlay) return;
    renderNodeIcon(target, state.modalIcon);
    state.modalTitle.textContent = payload.title || payload.label || 'Network node';
    state.modalSubtitle.textContent = payload.subtitle || '—';
    state.modalBadge.textContent = payload.badge || payload.kind || 'Info';
    state.modalBadge.className = `nm-detail-badge ${badgeToneClass(payload)}`;
    state.modalSummary.textContent = payload.summary || 'Network details';
    state.modalRows.innerHTML = rowsHtml(payload.rows);
    overlay.classList.add('is-visible');
    overlay.setAttribute('aria-hidden', 'false');
    document.body.classList.add('nm-modal-open');
  }

  function hideModal() {
    const overlay = ensureModal();
    if (!overlay) return;
    overlay.classList.remove('is-visible');
    overlay.setAttribute('aria-hidden', 'true');
    document.body.classList.remove('nm-modal-open');
  }

  function bindNode(node) {
    if (!node || node.dataset.nmBound === '1') return;
    node.dataset.nmBound = '1';
    node.addEventListener('mouseenter', () => showHover(node));
    node.addEventListener('focus', () => showHover(node));
    node.addEventListener('mouseleave', hideHover);
    node.addEventListener('blur', hideHover);
    node.addEventListener('mousemove', () => positionHoverCard(node));
    node.addEventListener('click', () => showModal(node));
    node.addEventListener('keydown', (event) => {
      if (event.key === 'Enter' || event.key === ' ') {
        event.preventDefault();
        showModal(node);
      }
    });
  }

  function bindAll() {
    document.querySelectorAll('[data-nm-node="1"]').forEach(bindNode);
  }

  function init() {
    ensureHoverCard();
    ensureModal();
    bindAll();
    document.addEventListener('keydown', (event) => {
      if (event.key === 'Escape') {
        hideHover();
        hideModal();
      }
    });
    const observer = new MutationObserver(() => bindAll());
    observer.observe(document.body, { childList: true, subtree: true });
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
