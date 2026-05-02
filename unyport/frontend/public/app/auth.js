// auth.js — CSRF + fetch wrapper + actions auth

let _csrf = null;

async function fetchCSRF() {
  try {
    const r = await fetch('/api/csrf?_=' + Date.now(), { credentials: 'include' });
    const d = await r.json();
    _csrf = d.csrf_token || d.unyport_csrf_token || null;
  } catch {
    _csrf = null;
  }
}

async function apiFetch(url, opts = {}) {
  const method = (opts.method || 'GET').toUpperCase();
  const headers = { ...(opts.headers || {}) };
  const write = ['POST', 'PUT', 'PATCH', 'DELETE'].includes(method);

  if (write && url !== '/api/login') {
    if (!_csrf) await fetchCSRF();
    if (_csrf) headers['X-CSRF-Token'] = _csrf;
  }

  const go = () => fetch(url, { ...opts, method, headers, credentials: 'include' });
  let res = await go();

  if (res.status === 403 && write) {
    await fetchCSRF();
    if (_csrf) headers['X-CSRF-Token'] = _csrf;
    res = await go();
  }

  if (res.status === 401) throw Object.assign(new Error('unauthorized'), { status: 401 });
  if (!res.ok) throw new Error('HTTP ' + res.status);

  const ct = res.headers.get('Content-Type') || '';
  if (res.status === 204 || !ct.includes('application/json')) return res;
  return res.json();
}

async function login(email, password) {
  await apiFetch('/api/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
  });
  await fetchCSRF();
}

async function logout() {
  try { await apiFetch('/api/logout', { method: 'POST' }); } catch { }
  _csrf = null;
}