// sse.js — SSE /sse/system

class SystemSSE {
  constructor(onSnapshot, onStatus) {
    this._es = null;
    this._onSnap = onSnapshot;
    this._onStatus = onStatus;
  }

  connect() {
    this.disconnect();
    const es = new EventSource('/sse/system', { withCredentials: true });
    this._es = es;
    es.onopen = () => this._onStatus(true);
    es.onmessage = (e) => { try { this._onSnap(JSON.parse(e.data)); } catch { } };
    es.onerror = () => this._onStatus(false);
  }

  disconnect() {
    if (this._es) { this._es.close(); this._es = null; }
    this._onStatus(false);
  }
}