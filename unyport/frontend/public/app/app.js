// app.js — point d'entrée Alpine
// Dépend de : utils.js, auth.js, sse.js, chart.js (chargés avant dans index.html)

document.addEventListener('alpine:init', () => {
  Alpine.data('unyport', () => ({

    // ── État ──
    page: 'loading',
    theme: localStorage.getItem('theme') || 'dark',
    email: '', password: '', loginLoading: false, loginError: '',
    sys: {}, uptime: '—', cpuPct: 0, memPct: 0, sseConnected: false,
    apps: [],
    _sse: null,
    netRxPct: 0,
    netTxPct: 0,
    netRxVal: '0 B/s',
    netTxVal: '0 B/s',
    userEmail: '',
    userName: '',
    userInitial: '?',

    // ── Computed ──
    get osLogo() { return osLogo(this.sys.os_release || this.sys.os); },
    get cpuLogo() { return cpuLogo(this.sys.cpu_vendor); },
    get cpuBarClass() { return this.cpuPct > 80 ? 'crit' : this.cpuPct > 60 ? 'warn' : ''; },
    get memBarClass() { return this.memPct > 85 ? 'crit' : this.memPct > 65 ? 'warn' : ''; },

    // ── Xen context ──
    get isXen() { return !!this.sys.xen_role; },
    get isDom0() { return this.sys.xen_role === 'Dom0'; },
    get isDomU() { return this.sys.xen_role === 'DomU'; },

    // Infos board : masquées sous Xen (DMI non fiable en VM)
    get showBoard() { return !this.isXen; },

    // Label plateforme affiché dans la card système
    get platformLabel() {
      if (this.isDom0) return 'Xen Dom0 — Hyperviseur';
      if (this.isDomU) return 'Xen DomU — Machine virtuelle';
      return null;
    },

    // ── Helpers exposés au template ──
    fmtFreq, fmtMB, fmtBytes, cleanCPU, appIcon,

    // ── Init (appelé automatiquement par Alpine) ──
    async init() {
      document.documentElement.setAttribute('data-theme', this.theme);
      await fetchCSRF();
      try {
        const s = await apiFetch('/api/session');
        if (s?.ok) { this._setUser(s.email || ''); await this._enterDashboard(); }
        else this.page = 'login';
      } catch {
        this.page = 'login';
      }
    },

    // ── Auth ──
    async doLogin() {
      this.loginError = '';
      this.loginLoading = true;
      try {
        await login(this.email, this.password);
        this._setUser(this.email);
        await this._enterDashboard();
      } catch (e) {
        this.loginError = e.status === 401 ? 'Invalid credentials' : (e.message || 'Login failed');
      } finally {
        this.loginLoading = false;
      }
    },

    async doLogout() {
      this._stopSSE();
      destroyCharts();
      await logout();
      this.page = 'login';
      this.sys = {}; this.apps = []; this.uptime = '—';
    },

    goSettings() { console.log('settings — TODO'); },

    _setUser(email) {
      this.userEmail = email || '';
      const parts = (email || '').split('@');
      this.userName = parts[0] || 'admin';
      this.userInitial = (parts[0] || 'A')[0].toUpperCase();
    },

    toggleTheme() {
      this.theme = this.theme === 'dark' ? 'light' : 'dark';
      document.documentElement.setAttribute('data-theme', this.theme);
      localStorage.setItem('theme', this.theme);
    },

    // ── Dashboard ──
    async _enterDashboard() {
      this.page = 'dashboard';
      await this._loadApps();
      await this._loadSysInfo();
      await this.$nextTick();
      if (window.FontAwesome) window.FontAwesome.dom.i2svg();
      initCharts();
      // Pré-remplit les charts avec la valeur courante
      if (this.sys.cpu_freq_avg_mhz) pushFreq(this.sys.cpu_freq_avg_mhz);
      if (this.sys.mem_used) pushMem(this.sys.mem_used);
      // Petit délai pour que le cookie soit bien enregistré par le navigateur
      // avant d'ouvrir la connexion EventSource
      setTimeout(() => this._startSSE(), 500);
    },

    async _loadApps() {
      try {
        const list = await apiFetch('/api/apps');
        this.apps = Array.isArray(list) ? list : [];
      } catch { this.apps = []; }
    },

    async _loadSysInfo() {
      try {
        const info = await apiFetch('/api/system');
        if (info) {
          this.sys = { ...this.sys, ...info };
          this.uptime = formatUptime(info.uptime);
        }
      } catch { }
    },

    // ── SSE ──
    _startSSE() {
      this._sse = new SystemSSE(
        (snap) => this._onSnapshot(snap),
        (connected) => { this.sseConnected = connected; },
      );
      this._sse.connect();
    },

    _stopSSE() { this._sse?.disconnect(); this._sse = null; },

    _onSnapshot(snap) {
      this.sys = {
        ...this.sys,
        cpu_usage: snap.cpu_usage,
        cpu_freq_avg_mhz: snap.cpu_freq_avg_mhz,
        cpu_freq_max_mhz: snap.cpu_freq_max_mhz,
        mem_total: snap.mem_total_mb,
        mem_used: snap.mem_used_mb,
        mem_free: snap.mem_free_mb,
        net_iface: snap.net_iface || this.sys.net_iface,
        net_ip: snap.net_ip || this.sys.net_ip,
        net_rx_bytes: snap.net_rx_bytes,
        net_tx_bytes: snap.net_tx_bytes,
        net_rx_bps: snap.net_rx_bps,
        net_tx_bps: snap.net_tx_bps,
      };
      this.cpuPct = Math.round(Math.max(0, Math.min(100, snap.cpu_usage || 0)));
      this.memPct = this.sys.mem_total > 0
        ? Math.round((this.sys.mem_used / this.sys.mem_total) * 100) : 0;
      document.documentElement.style.setProperty('--cpu-pct', this.cpuPct + '%');
      const cpuColor = this.cpuPct > 80 ? '#ff4444' : this.cpuPct > 60 ? '#ff9e3b' : '#5a5880';
      document.documentElement.style.setProperty('--cpu-color', cpuColor);
      document.documentElement.style.setProperty('--mem-pct', this.memPct + '%');
      // Couleur mémoire dynamique : vert → orange → rouge
      const memColor = this.memPct > 85 ? '#ff4444' : this.memPct > 65 ? '#ff9e3b' : '#22c9a0';
      document.documentElement.style.setProperty('--mem-color', memColor);
      updateGauge(this.cpuPct);
      _updateNetGauge(this, snap.net_rx_bps || 0, snap.net_tx_bps || 0);
      pushFreq(snap.cpu_freq_avg_mhz);
      pushMem(snap.mem_used_mb);
    },

  }));
});