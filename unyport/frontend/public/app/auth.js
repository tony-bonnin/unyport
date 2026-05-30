// auth.js — CSRF + fetch wrapper + actions auth

let _csrf = null;

async function fetchCSRF() {
  try {
    const r = await fetch('/api/csrf?_=' + Date.now(), {
      credentials: 'include',
      cache: 'no-store',
      headers: { 'Accept': 'application/json' },
    });
    const d = await r.json();
    _csrf = d.csrf_token || d.unyport_csrf_token || null;
  } catch {
    _csrf = null;
  }
}

async function apiFetch(url, opts = {}) {
  const method = (opts.method || 'GET').toUpperCase();
  const write = ['POST', 'PUT', 'PATCH', 'DELETE'].includes(method);

  const buildHeaders = async () => {
    const headers = { ...(opts.headers || {}) };
    if (opts.body && !headers['Content-Type']) headers['Content-Type'] = 'application/json';
    if (write && url !== '/api/login') {
      if (!_csrf) await fetchCSRF();
      if (_csrf) headers['X-CSRF-Token'] = _csrf;
    }
    return headers;
  };

  let headers = await buildHeaders();
  const go = () => fetch(url, { ...opts, method, headers, credentials: 'include', cache: 'no-store' });
  let res = await go();

  if (res.status === 403 && write) {
    _csrf = null;
    if (!_csrf) await fetchCSRF();
    headers = await buildHeaders();
    res = await go();
  }

  if (res.status === 423) throw Object.assign(new Error('password_change_required'), { status: 423 });
  if (res.status === 401) throw Object.assign(new Error('unauthorized'), { status: 401 });
  if (!res.ok) {
    let msg = 'HTTP ' + res.status;
    try { const e = await res.clone().json(); if (e && e.error) msg = e.error; } catch { }
    throw Object.assign(new Error(msg), { status: res.status });
  }

  const ct = res.headers.get('Content-Type') || '';
  if (res.status === 204 || !ct.includes('application/json')) return res;
  return res.json();
}

async function login(email, password) {
  const resp = await apiFetch('/api/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
  });
  // Effacer le flag logout — l'utilisateur se reconnecte volontairement.
  localStorage.removeItem('_logged_out');
  await fetchCSRF();
  return resp;
}

async function logout() {
  // Poser le flag AVANT la requête — si le fetch échoue (403 CSRF, réseau),
  // init() verra le flag et refusera la session au prochain chargement.
  localStorage.setItem('_logged_out', '1');
  _csrf = null;
  // Rafraîchir le CSRF — peut être périmé après longue session (MaxAge 3600s).
  // Sans token valide gorilla/csrf renvoie 403 et le Set-Cookie n'est jamais émis.
  await fetchCSRF();
  try {
    await apiFetch('/api/logout', { method: 'POST' });
  } catch (e) {
    console.warn('logout request failed:', e?.message);
  }
  _csrf = null;
}
