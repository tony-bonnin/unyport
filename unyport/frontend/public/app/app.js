// _ROUTES — constante module-level (hors state Alpine CSP)
const _ROUTES = {
  '/': { key: 'dashboard', role: 'viewer' },
  '/dashboard': { key: 'dashboard', role: 'viewer' },
  '/hypervisor': { key: 'hypervisor', role: 'viewer' },
  '/resources': { key: 'resources', role: 'viewer' },
  '/network': { key: 'network', role: 'viewer' },
  '/storage': { key: 'storage', role: 'viewer' },
  '/security': { key: 'security', role: 'viewer' },
  '/settings': { key: 'settings', role: 'viewer' },
};

// Mapping pour la mise à jour optimisée de l'état système (backend -> frontend)
const SYS_STATE_MAP = {
  os_release: 'sysOsRelease',
  kernel: 'sysKernel',
  date: 'sysDate',
  cpu_model: 'sysCpuModel',
  cpu_vendor: 'sysCpuVendor',
  cpu_cores: 'sysCpuCores',
  cpu_usage: 'sysCpuUsage',
  cpu_freq_avg_mhz: 'sysCpuFreqAvg',
  cpu_freq_max_mhz: 'sysCpuFreqMax',
  mem_total_mb: 'sysMemTotalMb',
  mem_used_mb: 'sysMemUsedMb',
  mem_free_mb: 'sysMemFreeMb',
  mem_cached_mb: 'sysMemCachedMb',
  net_iface: 'sysNetIface',
  net_ip: 'sysNetIp',
  net_rx_bytes: 'sysNetRxBytes',
  net_tx_bytes: 'sysNetTxBytes',
  net_rx_bps: 'sysNetRxBps',
  net_tx_bps: 'sysNetTxBps',
};

// app.js — TRINITY · point d'entrée Alpine.js (CSP build)
document.addEventListener('alpine:init', () => {
  Alpine.data('unyport', () => ({

    // ── État primitif ────────────────────────────────────────
    page: 'loading',
    currentPage: 'dashboard',
    theme: localStorage.getItem('theme') || 'light',

    email: '',
    password: '',
    loginPasswordVisible: false,
    loginLoading: false,
    loginError: '',

    uptime: '—',
    uptimeSecs: 0,
    _uptimeTimer: null,
    cpuPct: 0,
    memPct: 0,

    sseConnected: false,
    apps: [],
    vms: [],
    lbuPresent: false,
    lbuState: 'absent',
    lbuArchive: '',
    _sse: null,
    _postLoginTimer: null,

    // Remarque: netRxVal/netTxVal non assignés ici, gérés par _updateNetGauge si nécessaire
    netRxVal: '0 B/s',
    netTxVal: '0 B/s',

    userEmail: '',
    userName: '',
    userInitial: '?',
    userRole: '',
    userDisplayName: '',
    userAvatar: '',
    userPhotoUrl: '',
    userSSHKey: '',
    userDefaultCredentials: false,
    mustChangePassword: false,

    mustChangeOld: '',
    mustChangeNew: '',
    mustChangeConfirm: '',
    mustChangeError: '',
    mustChangeLoading: false,

    xenDomains: [],
    xenInfo: {},

    brandingLogoSrc: '',
    brandingHasLogo: false,
    brandingColors: { dom0: '#3e3aab', domu: '#a0284a', container: '#b05a1a', alpine: '#28587c' },

    brandingLogoBase64: '',
    brandingLogoUrl: '',
    brandingLogoUrlInput: '',
    brandingLogoPreview: '',
    brandingLogoMode: 'current',
    brandingLogoError: '',
    brandingColorDom0: '#3e3aab',
    brandingColorDomU: '#a0284a',
    brandingColorContainer: '#b05a1a',
    brandingColorAlpine: '#28587c',
    brandingSaving: false,
    brandingError: '',
    brandingSuccess: '',

    profilePage: 'info',
    profileOpen: false,
    dropdownOpen: false,
    mobileMenuOpen: false,
    profileSaving: false,
    profileError: '',
    profileSuccess: '',
    profileDisplayName: '',
    profileEmail: '',
    profileAvatar: '',
    profilePhotoUrl: '',
    profilePhotoMode: 'current',
    profilePhotoUrlInput: '',
    profilePhotoPreview: '',
    profilePhotoError: '',
    profileSSHKey: '',
    pwdOld: '',
    pwdNew: '',
    pwdConfirm: '',
    pwdError: '',
    pwdSuccess: '',

    adminUsersOpen: false,
    adminUsers: [],
    adminUsersLoading: false,
    adminNewEmail: '',
    adminNewPassword: '',
    adminNewRole: 'viewer',
    adminCreateError: '',

    sysHostname: '—',
    sysOsRelease: 'Alpine Linux',
    sysOsVersion: '',
    sysKernel: '—',
    sysDate: '—',
    sysCpuModel: '—',
    sysCpuVendor: '—',
    sysCpuCores: 0,
    sysCpuUsage: 0,
    sysCpuFreqAvg: 0,
    sysCpuFreqMax: 0,
    sysMemTotalMb: 0,
    sysMemUsedMb: 0,
    sysMemFreeMb: 0,
    sysNetIface: '—',
    sysNetIp: '—',
    sysNetRxBytes: 0,
    sysNetTxBytes: 0,
    sysNetRxBps: 0,
    sysNetTxBps: 0,

    sysDisks: [],
    sysLoadAvg: { load1: 0, load5: 0, load15: 0 },
    sysCPUTemps: [],
    sysTopProcs: [],
    sysMemCachedMb: 0,

    alpineLatestVer: '',
    alpineUpdateLevel: '',
    kernelLatestVer: '',
    kernelUpdateLevel: '',

    sysBoardName: '', sysBoardVendor: '', boardVersion: '',
    biosInfo: { bios_vendor: '', bios_version: '', bios_date: '', bios_year: '' },
    biosAvailable: false,
    gpuList: [], gpuCount: 0,
    apkCount: 0, apkDbExists: false, apkList: [], apkLoading: false, apkSearch: '', apkShowAll: false,
    modList: [], modCount: 0, modLoading: false, modSearch: '',
	    svcList: [], svcLoading: false, svcFilter: 'all',
	    svcStats: { started: 0, stopped: 0, crashed: 0, inactive: 0 },
	    logFiles: [], logSelectedFile: '', logLines: [], logLoading: false, logLinesCount: 100,
    securityLoading: false,
    securityError: '',
    securitySummary: { ok: 0, warn: 0, critical: 0, unknown: 0 },
    securityChecks: [],
    securityServices: [],
    securityProcesses: [],
    securityListeners: [],
    rebootHeatmapYear: new Date().getUTCFullYear(),
    rebootHeatmapLeadBlank: 0,
    rebootHeatmapDays: [],
    rebootHeatmapLoading: false,
    rebootHeatmapError: '',
    rebootHeatmapTotal: 0,
    rebootHeatmapMaxPerDay: 0,
    rebootHeatmapDetailOpen: false,
    rebootHeatmapDetail: null,

    hostRoleRole: '',
    hostRoleRuntime: '',
    hostRoleLabelStr: '—',
    hostRoleVerified: false,

    // ── Fonctions utilitaires ─────────────────────────────────
    // Optimisation: Liaison directe des fonctions externes au lieu de wrappers fléchés
    _fmtFreq: fmtFreq,
    _fmtMB: fmtMB,
    _fmtBytes: fmtBytes,
    _cleanCPU: cleanCPU,
    _formatUptime: formatUptime,
    _appIcon: appIcon,
    _xenRoleLabel: xenRoleLabel,

    _hostRoleIcon: (role) => {
      const icons = {
        'Dom0': 'fa-solid fa-cubes',
        'DomU': 'fa-solid fa-cube',
        'Container': 'fa-solid fa-box',
        'Alpine': 'fa-solid fa-server',
        'Unknown': 'fa-solid fa-circle-question',
      };
      return icons[role] || 'fa-solid fa-server';
    },

    // ── Getters ─────────────────────────────────────────────
    get cpuModelClean() { return this._cleanCPU(this.sysCpuModel); },
    get cpuFreqFmt() { return this._fmtFreq(this.sysCpuFreqAvg); },
    get cpuCoresDisplay() { return this.sysCpuCores || '—'; },
    get memUsedFmt() { return this._fmtMB(this.sysMemUsedMb); },
    get memTotalFmt() { return this._fmtMB(this.sysMemTotalMb); },
    get memFreeFmt() { return this._fmtMB(this.sysMemFreeMb); },
    get netIfaceDisplay() { return this.sysNetIface || '—'; },
    get netIpDisplay() { return this.sysNetIp || '—'; },

    get roleWithRuntime() {
      const role = this.hostRoleRole || '—';
      const rt = this.hostRoleRuntime;
      if (!rt || rt === 'native' || rt === '') return role;
      return `${role} · ${rt}`;
    },

    get kernelShort() {
      const k = this.sysKernel;
      return k.split('-')[0] || k || '—';
    },

    get kernelLts() {
      const k = this.sysKernel;
      if (!k || k === '—') return '—';
      return k.replace(/-\d+(-lts)$/, '$1') || k;
    },

    get alpineVerDisplay() {
      if (this.sysOsVersion) return `Alpine ${this.sysOsVersion}`;
      const m = (this.sysOsRelease || '').match(/(\d+\.\d+[\.\d]*)/);
      return m ? `Alpine ${m[1]}` : (this.sysOsRelease || 'Alpine Linux');
    },

    get alpineVerBadgeClass() {
      const lvl = this.alpineUpdateLevel;
      return `ver-badge ver-${lvl === 'error' ? 'error' : lvl ? lvl : 'checking'}`;
    },

    get alpineVerBadgeLabel() {
      const lvl = this.alpineUpdateLevel;
      if (!lvl) return '…';
      if (lvl === 'ok') return 'up to date';
      if (lvl === 'error') return 'unavailable';
      return this.alpineLatestVer ? `${this.alpineLatestVer} available` : lvl;
    },

    get kernelVerBadgeClass() {
      const lvl = this.kernelUpdateLevel;
      return lvl ? `ver-badge ver-${lvl === 'error' ? 'error' : lvl}` : '';
    },

    get kernelVerBadgeLabel() {
      const lvl = this.kernelUpdateLevel;
      if (!lvl) return '';
      if (lvl === 'ok') return 'up to date';
      if (lvl === 'error') return 'unavailable';
      return this.kernelLatestVer ? `${this.kernelLatestVer} available` : lvl;
    },

    get roleLabel() { return this.hostRoleLabelStr || '—'; },
    get roleIcon() { return this._hostRoleIcon(this.hostRoleRole); },
    get roleVerified() { return this.hostRoleVerified; },
    get runtimeName() { return this.hostRoleRuntime || '—'; },

    get statHostIcon() {
      const rt = this.hostRoleRuntime;
      const role = this.hostRoleRole;
      if (rt === 'docker') return 'fa-brands fa-docker';
      if (rt === 'podman') return 'fa-solid fa-box';
      if (rt === 'lxc' || rt === 'lxd') return 'fa-solid fa-cube';
      if (rt === 'xen' && role === 'Dom0') return 'icon-xen';
      if (rt === 'xen' && role === 'DomU') return 'icon-alpine';
      if (role === 'Dom0') return 'icon-xen';
      if (role === 'DomU') return 'icon-alpine';
      if (role === 'Container') return 'fa-solid fa-box';
      if (role === 'Alpine') return 'icon-alpine';
      return 'fa-solid fa-tag';
    },

    get showRuntime() {
      const r = this.hostRoleRuntime;
      return r !== '' && r !== 'native' && r !== '—';
    },

    get xenLabel() { return this.hostRoleLabelStr || '—'; },
    get isXenDom0() { return this.hostRoleRole === 'Dom0'; },
    get isXenDomU() { return this.hostRoleRole === 'DomU'; },
    get isInContainer() { return this.hostRoleRole === 'Container'; },
    get isBaremetal() { return this.hostRoleRole === 'Alpine'; },
    get isXen() { return this.isXenDom0 || this.isXenDomU; },
    get showXenDomains() { return this.isXenDom0 && this.xenDomains?.length > 0; },
    get showXenInfo() { return this.isXenDom0 && this.xenInfo?.available === true; },
    get xenCPUCount() { return Number(this.xenInfo?.cpus) || this.sysCpuCores || 0; },
    get xenMemoryUsedMB() {
      const total = Number(this.xenInfo?.total_memory_mb) || 0;
      const free = Number(this.xenInfo?.free_memory_mb) || 0;
      return total > free ? total - free : 0;
    },
    get xenMemoryPct() {
      const total = Number(this.xenInfo?.total_memory_mb) || 0;
      if (!total) return 0;
      return Math.round((this.xenMemoryUsedMB / total) * 100);
    },

    get panelHostTitle() {
      const r = this.hostRoleRole;
      if (r === 'Dom0') return 'Xen Hypervisor';
      if (r === 'DomU') return 'Virtual machine';
      if (r === 'Container') return 'Environment';
      return 'Alpine host';
    },

    get hostPageTitle() {
      const r = this.hostRoleRole;
      if (r === 'Dom0') return 'Hypervisor';
      if (r === 'DomU') return 'Virtual machine';
      if (r === 'Container') return 'Container';
      return 'Alpine host';
    },

    get hostPageSubtitle() {
      const r = this.hostRoleRole;
      if (r === 'Dom0') return 'Xen Dom0 hypervisor overview and domain status.';
      if (r === 'DomU') return 'Xen DomU virtual machine overview.';
      if (r === 'Container') return 'Container runtime overview.';
      return 'Alpine Linux host overview.';
    },

    get hostMenuTitle() { return 'Open ' + this.hostPageTitle.toLowerCase() + ' overview'; },

    get hostPageIcon() {
      const r = this.hostRoleRole;
      if (r === 'Dom0') return 'fa-solid fa-cubes';
      if (r === 'DomU') return 'fa-solid fa-cube';
      if (r === 'Container') return 'fa-solid fa-box';
      return 'fa-solid fa-server';
    },

    get cpuBarClass() {
      if (this.cpuPct > 80) return 'crit';
      if (this.cpuPct > 60) return 'warn';
      return '';
    },

    get memBarClass() {
      if (this.memPct > 85) return 'crit';
      if (this.memPct > 65) return 'warn';
      return '';
    },

    get showLbu() { return this.lbuPresent; },
    get showLbuDirty() { return this.lbuPresent && this.lbuState === 'dirty'; },
    get showLbuClean() { return this.lbuPresent && this.lbuState === 'clean'; },
    get lbuBoxClass() { return this.lbuState; },
    get lbuText() {
      if (this.lbuState === 'clean') return 'Committed';
      if (this.lbuState === 'dirty') return 'Uncommitted';
      return '—';
    },

    get loadAvgFmt() {
      const l = this.sysLoadAvg;
      if (!l) return '— — —';
      return `${l.load1.toFixed(2)}  ${l.load5.toFixed(2)}  ${l.load15.toFixed(2)}`;
    },

    get cpuTempFmt() {
      const t = this.sysCPUTemps;
      return (t && t.length) ? `${t[0].temp_c.toFixed(1)}°C` : '—';
    },

    get diskTotalFmt() {
      const total = this.sysDisks.reduce((s, d) => s + (d.total_mb || 0), 0);
      return this._fmtMB(total);
    },

    get svcFiltered() {
      return this.svcFilter === 'all' ? this.svcList : this.svcList.filter(s => s.state === this.svcFilter);
    },

    get modFiltered() {
      return !this.modSearch ? this.modList : this.modList.filter(m => m.name.toLowerCase().includes(this.modSearch.toLowerCase()));
    },

    get apkFiltered() {
      if (this.apkSearch) {
        const q = this.apkSearch.toLowerCase();
        return this.apkList.filter(p => p.name.toLowerCase().includes(q)).slice(0, 200);
      }
      return this.apkShowAll ? this.apkList : this.apkList.slice(0, 50);
    },

    get svcStartedCount() { return this.svcStats.started || 0; },
    get svcCrashedCount() { return this.svcStats.crashed || 0; },

    get lbuSubText() {
      if (this.lbuState === 'clean') return this.lbuArchive || 'Configuration OK';
      if (this.lbuState === 'dirty') return 'lbu commit required';
      return '';
    },

    get rebootHeatmapCells() {
      const blanks = Array.from({ length: this.rebootHeatmapLeadBlank || 0 }, (_, idx) => ({
        key: `blank-${idx}`,
        blank: true,
        gridRow: idx % 7,
      }));
      let sourceDays = Array.isArray(this.rebootHeatmapDays) ? this.rebootHeatmapDays : [];
      if (sourceDays.length < 300) {
        const year = Number(this.rebootHeatmapYear) || new Date().getUTCFullYear();
        const daysInYear = new Date(Date.UTC(year, 1, 29)).getUTCMonth() === 1 ? 366 : 365;
        const countsByDate = new Map(sourceDays.map((day) => [day.date, day]));
        sourceDays = [];
        for (let i = 0; i < daysInYear; i += 1) {
          const day = new Date(Date.UTC(year, 0, 1 + i));
          const date = day.toISOString().slice(0, 10);
          const existing = countsByDate.get(date);
          sourceDays.push(existing || {
            date,
            count: 0,
            level: 0,
            label: day.toLocaleDateString('en-GB', {
              weekday: 'short',
              day: '2-digit',
              month: 'short',
              year: 'numeric',
              timeZone: 'UTC',
            }),
          });
        }
      }
      const days = sourceDays.map((day, idx) => ({
        ...day,
        key: day.date,
        blank: false,
        gridRow: ((this.rebootHeatmapLeadBlank || 0) + idx) % 7,
      }));
      return blanks.concat(days);
    },

    get rebootHeatmapMonths() {
      const year = Number(this.rebootHeatmapYear) || new Date().getUTCFullYear();
      const lead = Number(this.rebootHeatmapLeadBlank) || 0;
      return Array.from({ length: 12 }, (_, monthIndex) => {
        const firstDay = new Date(Date.UTC(year, monthIndex, 1));
        const dayOfYear = Math.floor((firstDay - new Date(Date.UTC(year, 0, 1))) / 86400000);
        const weekColumn = Math.floor((lead + dayOfYear) / 7) + 1;
        return {
          key: `month-${monthIndex}`,
          label: firstDay.toLocaleDateString('en-GB', { month: 'short', timeZone: 'UTC' }),
          columnClass: `col-${weekColumn}`,
        };
      });
    },

    get rebootHeatmapSummaryLabel() {
      const total = Number(this.rebootHeatmapTotal) || 0;
      return `${total} restart${total > 1 ? 's' : ''}`;
    },

    get rebootHeatmapToday() {
      return new Date().toISOString().slice(0, 10);
    },

    get rebootHeatmapDetailCountLabel() {
      const detail = this.rebootHeatmapDetail || {};
      const count = Number(detail.count) || 0;
      return `${count} restart${count > 1 ? 's' : ''}`;
    },

    get rebootHeatmapDetailLabel() {
      const detail = this.rebootHeatmapDetail || {};
      return detail.label || detail.date || '—';
    },

    get rebootHeatmapDetailDate() {
      const detail = this.rebootHeatmapDetail || {};
      return detail.date || '—';
    },

    get rebootHeatmapDetailLevel() {
      const detail = this.rebootHeatmapDetail || {};
      return Number(detail.level) || 0;
    },

    get rebootHeatmapDetailIsTodayLabel() {
      const detail = this.rebootHeatmapDetail || {};
      return detail.date === this.rebootHeatmapToday ? 'Yes' : 'No';
    },

    // ── Actions UI ───────────────────────────────────────────
    toggleDropdown() { this.dropdownOpen = !this.dropdownOpen; },
    closeDropdown() { this.dropdownOpen = false; },
    openMobileMenu() { this.mobileMenuOpen = true; },
    closeMobileMenu() { this.mobileMenuOpen = false; },
    navigateMobile(path) { this.navigate(path); this.mobileMenuOpen = false; },
    openProfileMenu() { this.closeDropdown(); this.openProfile(); },
    toggleThemeMenu() { this.closeDropdown(); this.toggleTheme(); },
    openAdminUsersMenu() { this.closeDropdown(); this.openAdminUsers(); },
    openProfileAndCloseMobileMenu() {
      this.closeMobileMenu();
      this.openProfile();
    },
    openAdminUsersAndCloseMobileMenu() {
      this.closeMobileMenu();
      this.openAdminUsers();
    },

    // ── Branding ─────────────────────────────────────────────
    async _loadBranding() {
      try {
        const b = await apiFetch('/api/branding');
        if (!b) return;
        this.brandingLogoSrc = b.logo_src || '';
        this.brandingHasLogo = b.has_logo || false;

        if (b.colors) {
          this.brandingColors = b.colors;
          this.brandingColorDom0 = b.colors.dom0 || '#3e3aab';
          this.brandingColorDomU = b.colors.domu || '#a0284a';
          this.brandingColorContainer = b.colors.container || '#b05a1a';
          this.brandingColorAlpine = b.colors.alpine || '#28587c';
        }
        this._applyBrandingCSS();
      } catch { /* Ignore, valeurs par défaut */ }
    },

    _applyBrandingCSS() {
      const r = document.documentElement;
      r.style.setProperty('--role-dom0', this.brandingColorDom0);
      r.style.setProperty('--role-domu', this.brandingColorDomU);
      r.style.setProperty('--role-container', this.brandingColorContainer);
      r.style.setProperty('--role-alpine', this.brandingColorAlpine);
      this.updateSwatches();
    },

    updateSwatches() {
      const map = {
        'swatch-dom0': this.brandingColorDom0,
        'swatch-domu': this.brandingColorDomU,
        'swatch-container': this.brandingColorContainer,
        'swatch-alpine': this.brandingColorAlpine,
      };
      for (const [id, color] of Object.entries(map)) {
        const el = document.getElementById(id);
        if (el) el.style.setProperty('background', color);
      }
    },

    initBrandingEditor() {
      this.$nextTick(() => this.updateSwatches());
      this.brandingLogoUrl = this.brandingLogoSrc;
      this.brandingLogoUrlInput = this.brandingLogoSrc.startsWith('http') ? this.brandingLogoSrc : '';
      this.brandingLogoBase64 = this.brandingLogoSrc.startsWith('data:') ? this.brandingLogoSrc : '';
      this.brandingLogoPreview = this.brandingLogoSrc;
      this.brandingLogoMode = 'current';
      this.brandingLogoError = this.brandingError = this.brandingSuccess = '';
    },

    _denyBrandingEdit() {
      this.brandingSuccess = '';
      this.brandingError = 'Branding changes require an administrator role.';
    },

    setBrandingLogoFromFile(event) {
      this.brandingLogoError = '';
      if (!this.canEditBranding) {
        if (event?.target) event.target.value = '';
        this._denyBrandingEdit();
        return;
      }
      const file = event.target.files?.[0];
      if (!file) return;
      if (!file.type.startsWith('image/')) return this.brandingLogoError = 'File must be an image (PNG, SVG, WebP…)';
      if (file.size > 2 * 1024 * 1024) return this.brandingLogoError = 'Image must be under 2 MB.';

      const reader = new FileReader();
      reader.onload = (e) => {
        this.brandingLogoPreview = e.target.result;
        this.brandingLogoBase64 = e.target.result;
        this.brandingLogoUrl = '';
      };
      reader.readAsDataURL(file);
    },

    applyBrandingLogoUrl() {
      this.brandingLogoError = '';
      if (!this.canEditBranding) {
        this._denyBrandingEdit();
        return;
      }
      const url = this.brandingLogoUrlInput.trim();
      if (!url) {
        this.brandingLogoPreview = this.brandingLogoBase64 = this.brandingLogoUrl = '';
        return;
      }
      if (!url.startsWith('http://') && !url.startsWith('https://')) return this.brandingLogoError = 'URL must start with http:// or https://';

      this.brandingLogoPreview = url;
      this.brandingLogoUrl = url;
      this.brandingLogoBase64 = '';
    },

    removeBrandingLogo() {
      if (!this.canEditBranding) {
        this._denyBrandingEdit();
        return;
      }
      this.brandingLogoPreview = this.brandingLogoBase64 = this.brandingLogoUrl = this.brandingLogoUrlInput = this.brandingLogoError = '';
    },

    async saveBranding() {
      if (!this.canEditBranding) {
        this._denyBrandingEdit();
        return;
      }
      this.brandingSaving = true;
      this.brandingError = this.brandingSuccess = '';
      try {
        await apiFetch('/api/branding/update', {
          method: 'PATCH',
          body: JSON.stringify({
            logo_base64: this.brandingLogoBase64,
            logo_url: this.brandingLogoUrl,
            colors: {
              dom0: this.brandingColorDom0,
              domu: this.brandingColorDomU,
              container: this.brandingColorContainer,
              alpine: this.brandingColorAlpine,
            },
          }),
        });
        this.brandingLogoSrc = this.brandingLogoBase64 || this.brandingLogoUrl;
        this.brandingColors = {
          dom0: this.brandingColorDom0, domu: this.brandingColorDomU,
          container: this.brandingColorContainer, alpine: this.brandingColorAlpine,
        };
        this._applyBrandingCSS();
        this.brandingSuccess = 'Branding saved.';
      } catch (e) {
        this.brandingError = e?.message || 'Save failed.';
      } finally {
        this.brandingSaving = false;
      }
    },

    async resetBranding() {
      if (!this.canEditBranding) {
        this._denyBrandingEdit();
        return;
      }
      if (!confirm('Reset branding to defaults?')) return;
      try {
        await apiFetch('/api/branding/reset', { method: 'DELETE' });
        this.brandingLogoSrc = this.brandingLogoPreview = this.brandingLogoBase64 = this.brandingLogoUrl = '';
        this.brandingColorDom0 = '#3e3aab';
        this.brandingColorDomU = '#a0284a';
        this.brandingColorContainer = '#b05a1a';
        this.brandingColorAlpine = '#28587c';
        this._applyBrandingCSS();
        this.brandingSuccess = 'Branding reset to defaults.';
      } catch (e) {
        this.brandingError = e?.message || 'Reset failed.';
      }
    },

    // ── Routeur SPA ─────────────────────────────────────────
    navigate(path) {
      path = this._authorizedPath(path);
      const key = (_ROUTES[path] || _ROUTES['/dashboard']).key;
      history.pushState({ page: key }, '', path);
      this._applyRoute(key);
    },

    _authorizedPath(path) {
      const route = _ROUTES[path] || _ROUTES['/dashboard'];
      if (route.role === 'admin' && !this.isAdmin) return '/dashboard';
      if (route.role === 'operator' && !this.isOperator) return '/dashboard';
      return _ROUTES[path] ? path : '/dashboard';
    },

    _applyRoute(key) {
      this.currentPage = key;
      if (key === 'dashboard') this.$nextTick(() => typeof initCharts === 'function' && initCharts());
      if (key === 'network') {
        this.$nextTick(() => {
          if (typeof renderNetworkMap === 'function' && window._nmLastArgs) {
            const a = window._nmLastArgs;
            renderNetworkMap(a[0], a[1], a[2], a[3], a[4], 'netmap-container');
          }
        });
      }
      if (key === 'settings') this.initBrandingEditor();
      if (key === 'security') this.loadSecurity();
      const ws = document.querySelector('.main-scroll');
      if (ws) ws.scrollTop = 0;
    },

    _initRouter() {
      window.addEventListener('popstate', (e) => {
        const path = this._authorizedPath(window.location.pathname);
        const key = (_ROUTES[path] || _ROUTES['/dashboard']).key;
        if (path !== window.location.pathname) history.replaceState({ page: key }, '', path);
        this._applyRoute(key);
      });
      const path = this._authorizedPath(window.location.pathname);
      const route = _ROUTES[path] || _ROUTES['/dashboard'];
      history.replaceState({ page: route.key }, '', path);
      this._applyRoute(route.key);
    },

    scrollTo(id) {
      const el = document.getElementById(id);
      if (el) el.scrollIntoView({ behavior: 'smooth', block: 'start' });
    },

    get themeToggleLabel() { return this.theme === 'dark' ? 'Light theme' : 'Dark theme'; },
    get themeIsDark() { return this.theme === 'dark'; },
    get themeIsLight() { return this.theme === 'light'; },

    get isAdmin() { return this.userRole === 'admin'; },
    get isOperator() { return this.userRole === 'admin' || this.userRole === 'operator' || this.userRole === 'viewer'; },
    get isViewer() { return this.userRole === 'viewer'; },
    get canEditBranding() { return this.isAdmin; },

    get brandingLogoEffective() { return this.brandingLogoPreview || this.brandingLogoSrc || ''; },
    get brandingHasLogoPreview() { return this.brandingLogoEffective !== ''; },

    get onDashboard() { return this.currentPage === 'dashboard'; },
    get onHypervisor() { return this.currentPage === 'hypervisor'; },
    get onResources() { return this.currentPage === 'resources'; },
    get onNetwork() { return this.currentPage === 'network'; },
    get onStorage() { return this.currentPage === 'storage'; },
    get onSecurity() { return this.currentPage === 'security'; },
    get onSettings() { return this.currentPage === 'settings'; },

    getPageTitle() {
      const titles = {
        hypervisor: this.hostPageTitle, resources: 'Host Resources', network: 'Network',
        storage: 'Storage', security: 'Security', settings: 'Settings'
      };
      return titles[this.currentPage] || 'Dashboard';
    },

    get roleBadgeClass() {
      const cls = { admin: 'role-admin', operator: 'role-operator' };
      return cls[this.userRole] || 'role-viewer';
    },
    get roleBadgeLabel() {
      const lbl = { admin: 'Admin', operator: 'Operator' };
      return lbl[this.userRole] || 'Viewer';
    },
    get roleStickerLabel() {
      const lbl = { admin: 'A', operator: 'O' };
      return lbl[this.userRole] || 'V';
    },
    securityStatusClass(status) {
      return 'security-status-' + (status || 'unknown');
    },
    securityIconClass(status) {
      const icons = {
        ok: 'fa-solid fa-circle-check',
        warn: 'fa-solid fa-triangle-exclamation',
        critical: 'fa-solid fa-circle-xmark',
        unknown: 'fa-solid fa-circle-question',
      };
      return icons[status] || icons.unknown;
    },
    pct1(value) {
      const n = Number(value) || 0;
      return n.toFixed(1) + '%';
    },
    get userAvatarDisplay() { return this.userAvatar || this.userInitial || '?'; },
    get hasPhoto() { return this.userPhotoUrl !== '' && this.userPhotoUrl !== null; },
    get photoPreviewSrc() { return this.profilePhotoPreview || this.userPhotoUrl || ''; },
    get hasPhotoPreview() { return this.photoPreviewSrc !== ''; },

    // ── Init ─────────────────────────────────────────────────
    async init() {
      document.documentElement.setAttribute('data-theme', this.theme);
      if (window.__unyportSyncThemeColor) {
        window.__unyportSyncThemeColor(this.theme, this.hostRoleRole || 'Dom0');
      }
      this._updateThemeMetaColor(this.hostRoleRole || 'Dom0');

      // Optimisation: Parallélisation des appels indépendants
      await Promise.all([fetchCSRF(), this._loadBranding()]);

      if (localStorage.getItem('_logged_out') === '1') {
        this._showLogin();
        return;
      }
      try {
        const session = await apiFetch('/api/session');
        if (session?.ok) {
          this._setUser(session);
          if (this.mustChangePassword) { this.page = 'must_change'; return; }
          await this._enterDashboard();
          return;
        }
        this._showLogin();
      } catch {
        this._showLogin();
      }
    },

    // ── Auth ─────────────────────────────────────────────────
    async doLogin() {
      this.loginError = '';
      this.loginLoading = true;
      try {
        await login(this.email, this.password);
        const session = await apiFetch('/api/session');
        if (session?.ok) this._setUser(session);
        if (this.mustChangePassword) { this.page = 'must_change'; return; }
        await this._enterDashboard();
      } catch (e) {
        if (e?.status === 401) this.loginError = 'Invalid email or password';
        else if (e?.status === 429) this.loginError = 'Too many attempts - try again in 1 minute';
        else this.loginError = 'Unable to sign in';
      } finally {
        this.loginLoading = false;
      }
    },

    async doForcedPasswordChange() {
      this.mustChangeError = '';
      if (this.mustChangeNew.length < 8) return this.mustChangeError = 'Minimum 8 caractères.';
      if (this.mustChangeNew !== this.mustChangeConfirm) return this.mustChangeError = 'Les mots de passe ne correspondent pas.';

      this.mustChangeLoading = true;
      try {
        await apiFetch('/api/profile/password', {
          method: 'POST',
          body: JSON.stringify({ old_password: this.mustChangeOld, new_password: this.mustChangeNew }),
        });
        this.mustChangePassword = false;
        this.userDefaultCredentials = false;
        this.mustChangeOld = this.mustChangeNew = this.mustChangeConfirm = '';
        await this._enterDashboard();
      } catch (e) {
        this.mustChangeError = e?.message || 'Échec du changement.';
      } finally {
        this.mustChangeLoading = false;
      }
    },

    async doLogout(event) {
      if (event && typeof event.preventDefault === 'function') {
        event.preventDefault();
      }
      this.dropdownOpen = false;
      this.mobileMenuOpen = false;
      this._stopSSE();
      if (typeof destroyCharts === 'function') destroyCharts();
      try {
        await logout();
      } finally {
        this._resetState();
        this._showLogin();
        this._updateThemeMetaColor('Dom0');
        setTimeout(() => {
          if (window.location.pathname !== '/') {
            history.replaceState({}, '', '/');
          }
          if (this.page !== 'login') {
            this._showLogin();
            this._updateThemeMetaColor('Dom0');
          }
          window.location.replace('/');
        }, 120);
      }
    },

    toggleTheme() {
      this.theme = this.theme === 'dark' ? 'light' : 'dark';
      document.documentElement.setAttribute('data-theme', this.theme);
      localStorage.setItem('theme', this.theme);
      this._updateThemeMetaColor(this.hostRoleRole || 'Dom0');
      if (window.__unyportSyncThemeColor) {
        window.__unyportSyncThemeColor(this.theme, this.hostRoleRole || 'Dom0');
      } else {
        var evt;
        if (typeof window.CustomEvent === 'function') {
          evt = new CustomEvent('themechange');
        } else if (document.createEvent) {
          evt = document.createEvent('Event');
          evt.initEvent('themechange', false, false);
        } else {
          evt = null;
        }
        if (evt && window.dispatchEvent) {
          window.dispatchEvent(evt);
        } else if (evt && window.fireEvent) {
          window.fireEvent('on' + evt.type);
        }
      }

    },

    // ── Profil ───────────────────────────────────────────────
    openProfile() {
      this.profileDisplayName = this.userDisplayName;
      this.profileEmail = this.userEmail;
      this.profileAvatar = this.userAvatar;
      this.profileSSHKey = this.userSSHKey;
      this.profilePhotoUrl = this.userPhotoUrl;
      this.profilePhotoPreview = '';
      this.profilePhotoMode = 'current';
      this.profilePhotoUrlInput = this.userPhotoUrl;
      this.profilePhotoError = '';
      this.profilePage = 'info';
      this.profileError = this.profileSuccess = '';
      this.pwdOld = this.pwdNew = this.pwdConfirm = '';
      this.pwdError = this.pwdSuccess = '';
      this.profileOpen = true;
    },

    closeProfile() { this.profileOpen = false; },

    _denyViewerProfileAction() {
      this.profileSuccess = '';
      this.pwdSuccess = '';
      this.profileError = 'Viewer role is read-only.';
      this.pwdError = 'Viewer role is read-only.';
    },

    async saveProfile() {
      if (this.isViewer) {
        this._denyViewerProfileAction();
        return;
      }
      this.profileSaving = true;
      this.profileError = this.profileSuccess = '';
      try {
        await apiFetch('/api/profile', {
          method: 'PATCH',
          body: JSON.stringify({
            email: this.profileEmail,
            display_name: this.profileDisplayName,
            avatar: this.profileAvatar,
            photo_url: this.profilePhotoUrl,
            ssh_key: this.profileSSHKey,
          }),
        });
        const profile = await apiFetch('/api/profile');
        this._setUser(profile);
        this.profileEmail = this.userEmail;
        this.userDisplayName = this.profileDisplayName;
        this.userAvatar = this.profileAvatar;
        this.userSSHKey = this.profileSSHKey;
        this.userPhotoUrl = this.profilePhotoUrl;
        this._setUserName(this.userDisplayName || this.userEmail);
        this.profileSuccess = 'Profile updated.';
      } catch (e) {
        this.profileError = e?.message || 'Save failed.';
      } finally {
        this.profileSaving = false;
      }
    },

    setPhotoFromFile(event) {
      this.profilePhotoError = '';
      if (this.isViewer) {
        if (event?.target) event.target.value = '';
        this._denyViewerProfileAction();
        return;
      }
      const file = event.target.files?.[0];
      if (!file) return;
      if (!file.type.startsWith('image/')) return this.profilePhotoError = 'File must be an image (PNG, JPG, WebP…)';
      if (file.size > 2 * 1024 * 1024) return this.profilePhotoError = 'Image must be under 2 MB.';

      const reader = new FileReader();
      reader.onload = (e) => {
        this.profilePhotoPreview = e.target.result;
        this.profilePhotoUrl = e.target.result;
        this.profilePhotoMode = 'current';
      };
      reader.readAsDataURL(file);
    },

    applyPhotoUrl() {
      this.profilePhotoError = '';
      if (this.isViewer) {
        this._denyViewerProfileAction();
        return;
      }
      const url = this.profilePhotoUrlInput.trim();
      if (!url) {
        this.profilePhotoPreview = this.profilePhotoUrl = '';
        this.profilePhotoMode = 'current';
        return;
      }
      if (!url.startsWith('http://') && !url.startsWith('https://')) return this.profilePhotoError = 'URL must start with http:// or https://';
      this.profilePhotoPreview = url;
      this.profilePhotoUrl = url;
      this.profilePhotoMode = 'current';
    },

    removePhoto() {
      if (this.isViewer) {
        this._denyViewerProfileAction();
        return;
      }
      this.profilePhotoPreview = this.profilePhotoUrl = this.profilePhotoUrlInput = this.profilePhotoError = '';
      this.profilePhotoMode = 'current';
    },

    async savePassword() {
      if (this.isViewer) {
        this._denyViewerProfileAction();
        return;
      }
      this.pwdError = this.pwdSuccess = '';
      if (this.pwdNew !== this.pwdConfirm) return this.pwdError = 'Passwords do not match.';
      if (this.pwdNew.length < 8) return this.pwdError = 'Minimum 8 characters.';

      try {
        await fetchCSRF();
        await apiFetch('/api/profile/password', {
          method: 'POST',
          body: JSON.stringify({ old_password: this.pwdOld, new_password: this.pwdNew }),
        });
        this.pwdOld = this.pwdNew = this.pwdConfirm = '';
        this.userDefaultCredentials = false;
        this.pwdSuccess = 'Password changed.';
      } catch (e) {
        this.pwdError = e?.message || 'Change failed.';
      }
    },

    // ── Admin users ──────────────────────────────────────────
    async openAdminUsers() {
      this.adminUsersOpen = true;
      this.adminUsersLoading = true;
      this.adminCreateError = '';
      try {
        const list = await apiFetch('/api/admin/users');
        this.adminUsers = Array.isArray(list) ? list : [];
      } catch { this.adminUsers = []; }
      finally { this.adminUsersLoading = false; }
    },

    closeAdminUsers() { this.adminUsersOpen = false; },

    async adminCreateUser() {
      this.adminCreateError = '';
      try {
        await apiFetch('/api/admin/users', {
          method: 'POST',
          body: JSON.stringify({
            email: this.adminNewEmail,
            password: this.adminNewPassword,
            role: this.adminNewRole,
          }),
        });
        this.adminNewEmail = this.adminNewPassword = '';
        this.adminNewRole = 'viewer';
        await this.openAdminUsers();
      } catch (e) {
        this.adminCreateError = e?.message || 'Create failed.';
      }
    },

    async adminDeleteUser(email) {
      if (!confirm(`Delete ${email}?`)) return;
      try {
        await apiFetch(`/api/admin/users/${encodeURIComponent(email)}`, { method: 'DELETE' });
        await this.openAdminUsers();
      } catch (e) {
        alert(e?.message || 'Delete failed.');
      }
    },

    async adminSetRole(email, role) {
      try {
        await apiFetch(`/api/admin/users/${encodeURIComponent(email)}`, {
          method: 'PATCH',
          body: JSON.stringify({ role }),
        });
        await this.openAdminUsers();
      } catch (e) {
        alert(e?.message || 'Role change failed.');
      }
    },

    // ── Dashboard ────────────────────────────────────────────
    async _enterDashboard() {
      this.page = 'app';
      document.documentElement.removeAttribute('data-auth-page');
      this._lockShellScroll(false);
      await Promise.all([this._loadApps(), this._loadSysInfo(), this.loadRebootHistory()]);
      this._initRouter();

      if (window.FontAwesome?.dom) window.FontAwesome.dom.i2svg();

      if (this._postLoginTimer) clearTimeout(this._postLoginTimer);
      this._postLoginTimer = setTimeout(() => {
        this._postLoginTimer = null;
        if (this.page !== 'app') return;
        this._startSSE();
        this._loadExtended();
      }, 500);
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
        if (!info) return;

        this._applyHostRole(info.host_role, info.xen_role);

        // Application optimisée via SYS_STATE_MAP
        this._applySys({
          os_release: info.os_release,
          kernel: info.kernel,
          date: info.date,
          cpu_model: info.cpu_model,
          cpu_vendor: info.cpu_vendor,
          cpu_cores: info.cpu_cores,
          cpu_usage: info.cpu_usage,
          cpu_freq_avg_mhz: info.cpu_freq_avg_mhz,
          cpu_freq_max_mhz: info.cpu_freq_max_mhz,
          mem_total_mb: info.mem_total_mb ?? info.mem_total,
          mem_used_mb: info.mem_used_mb ?? info.mem_used,
          mem_free_mb: info.mem_free_mb ?? info.mem_free,
          net_iface: info.net_iface,
          net_ip: info.net_ip,
        });

        if (info.hostname) this.sysHostname = info.hostname;
        if (info.os_version) { this.sysOsVersion = info.os_version; this._checkVersions(info.os_version); }
        this._startUptimeTicker(info.uptime);
        this._updateLBU(info);
        if (info.disks) this.sysDisks = info.disks;
        if (info.load_avg) this.sysLoadAvg = info.load_avg;
        if (info.cpu_temps) this.sysCPUTemps = info.cpu_temps;
        if (info.top_procs) this.sysTopProcs = info.top_procs;
        if (info.xen_info) this.xenInfo = info.xen_info;
        if (info.xen_domains) this.xenDomains = info.xen_domains;
        if (info.board_name) this.sysBoardName = info.board_name;
        if (info.board_vendor) this.sysBoardVendor = info.board_vendor;
        if (info.board_version) this.boardVersion = info.board_version;
        if (info.bios) { this.biosInfo = info.bios; this.biosAvailable = !!info.bios.bios_vendor; }
        if (info.gpus) { this.gpuList = info.gpus; this.gpuCount = info.gpus.length; }
        if (info.apk) { this.apkCount = info.apk.count || 0; this.apkDbExists = info.apk.db_exists || false; }
        if (info.modules) this.modCount = info.modules.count || 0;
      } catch { /* SSE relais */ }
    },

    // ── Loaders portage ACF ──────────────────────────────────
    async loadServices() {
      if (this.svcLoading) return;
      this.svcLoading = true;
      try {
        const r = await apiFetch('/api/services');
        if (r) { this.svcList = r.services || []; this.svcStats = r.stats || {}; }
      } catch { this.svcList = []; }
      finally { this.svcLoading = false; }
    },

    async loadModules() {
      if (this.modLoading) return;
      this.modLoading = true;
      try {
        const r = await apiFetch('/api/modules');
        if (r) { this.modList = r.modules || []; this.modCount = r.count || 0; }
      } catch { this.modList = []; }
      finally { this.modLoading = false; }
    },

    async loadAPK() {
      if (this.apkLoading) return;
      this.apkLoading = true; this.apkShowAll = false;
      try {
        const r = await apiFetch('/api/packages');
        if (r) { this.apkList = r.packages || []; this.apkCount = r.count || 0; this.apkDbExists = r.db_exists || false; }
      } catch { this.apkList = []; }
      finally { this.apkLoading = false; }
    },

    async loadLogs() {
      try {
        const r = await apiFetch('/api/logs');
        this.logFiles = Array.isArray(r) ? r : [];
        if (this.logFiles.length > 0 && !this.logSelectedFile) this.logSelectedFile = this.logFiles[0].name;
      } catch { this.logFiles = []; }
    },

    async loadRebootHistory() {
      if (this.rebootHeatmapLoading) return;
      this.rebootHeatmapLoading = true;
      this.rebootHeatmapError = '';
      try {
        const r = await apiFetch(`/api/reboots?_=${Date.now()}`);
        this.rebootHeatmapYear = Number(r?.year) || new Date().getUTCFullYear();
        this.rebootHeatmapLeadBlank = Number(r?.lead_blank) || 0;
        this.rebootHeatmapDays = Array.isArray(r?.days) ? r.days : [];
        this.rebootHeatmapTotal = Number(r?.total_reboots) || 0;
        this.rebootHeatmapMaxPerDay = Number(r?.max_per_day) || 0;
      } catch (e) {
        this.rebootHeatmapDays = [];
        this.rebootHeatmapTotal = 0;
        this.rebootHeatmapMaxPerDay = 0;
        this.rebootHeatmapLeadBlank = 0;
        this.rebootHeatmapError = e?.message || 'Startup history unavailable.';
      } finally {
        this.rebootHeatmapLoading = false;
      }
    },

    rebootHeatmapCellClass(cell) {
      if (!cell || cell.blank) return 'reboot-heatmap-cell is-blank';
      const classes = ['reboot-heatmap-cell', `level-${Number(cell.level) || 0}`];
      if (cell.date === this.rebootHeatmapToday) classes.push('is-today');
      return classes.join(' ');
    },

    rebootHeatmapCellTitle(cell) {
      if (!cell || cell.blank) return '';
      return cell.label || `${cell.date}: ${cell.count || 0} restart${(cell.count || 0) > 1 ? 's' : ''}`;
    },

    openRebootHeatmapDetail(cell) {
      if (!cell || cell.blank) return;
      this.rebootHeatmapDetail = cell;
      this.rebootHeatmapDetailOpen = true;
    },

    closeRebootHeatmapDetail() {
      this.rebootHeatmapDetailOpen = false;
      this.rebootHeatmapDetail = null;
    },

    async loadSecurity() {
      if (this.securityLoading) return;
      this.securityLoading = true;
      this.securityError = '';
      try {
        const r = await apiFetch('/api/security');
        if (r) {
          this.securitySummary = r.summary || { ok: 0, warn: 0, critical: 0, unknown: 0 };
          this.securityChecks = r.checks || [];
          this.securityServices = r.services || [];
          this.securityProcesses = r.processes || [];
          this.securityListeners = r.listeners || [];
        }
      } catch (e) {
        this.securityError = e?.message || 'Security status unavailable.';
      } finally {
        this.securityLoading = false;
      }
    },

    async tailLog(filename) {
      if (!filename) return;
      this.logSelectedFile = filename;
      this.logLoading = true; this.logLines = [];
      try {
        const r = await apiFetch('/api/logs/tail?file=' + encodeURIComponent(filename) + '&n=' + this.logLinesCount);
        this.logLines = r?.lines || [];
      } catch { this.logLines = []; }
      finally { this.logLoading = false; }
    },

    // ── Vérification versions ────────────────────────────────
    async _checkVersions(currentAlpineVer) {
      if (!currentAlpineVer) return;
      try {
        const data = await apiFetch('/api/versions');
        if (!data) throw new Error('empty');
        if (data.kernel_lts) this._compareKernel(data.kernel_lts); else this.kernelUpdateLevel = 'error';
        if (data.alpine) this._compareAlpine(currentAlpineVer, data.alpine); else this.alpineUpdateLevel = 'error';
      } catch {
        this.kernelUpdateLevel = 'error';
        this.alpineUpdateLevel = 'error';
      }
    },

    _compareAlpine(current, latest) {
      if (!latest) { this.alpineUpdateLevel = 'ok'; return; }
      const [cM, cm, cp = 0] = current.split('.').map(Number);
      const [lM, lm, lp = 0] = latest.split('.').map(Number);
      this.alpineLatestVer = latest;
      if (lM > cM) this.alpineUpdateLevel = 'major';
      else if (lm > cm) this.alpineUpdateLevel = 'minor';
      else if (lp > cp) this.alpineUpdateLevel = 'patch';
      else this.alpineUpdateLevel = 'ok';
    },

    _compareKernel(latestLts) {
      if (!latestLts) return;
      const cur = this.kernelLts;
      if (!cur || cur === '—') return;
      const parseKer = v => v.replace(/-lts$/, '').split('.').map(Number);
      const [cM, cm, cp = 0] = parseKer(cur);
      const [lM, lm, lp = 0] = parseKer(latestLts);
      this.kernelLatestVer = latestLts;
      if (lM > cM) this.kernelUpdateLevel = 'major';
      else if (lm > cm) this.kernelUpdateLevel = 'minor';
      else if (lp > cp) this.kernelUpdateLevel = 'patch';
      else this.kernelUpdateLevel = 'ok';
    },

    _loadExtended() {
      Promise.all([this.loadServices(), this.loadLogs()]).catch(() => { });
    },

    // ── SSE ──────────────────────────────────────────────────
    _startSSE() {
      if (this._sse) return;
      this._sse = new SystemSSE(
        (snap) => this._onSnapshot(snap),
        (ok) => { this.sseConnected = ok; },
      );
      this._sse.connect();
    },

    _stopSSE() {
      if (this._postLoginTimer) {
        clearTimeout(this._postLoginTimer);
        this._postLoginTimer = null;
      }
      this._sse?.disconnect();
      this._sse = null;
    },

    _onSnapshot(snap) {
      if (!snap) return;

      this._applySys(snap);
      this._updateLBU(snap);

      // Resync uptime
      if (snap.uptime) {
        const s = parseInt(String(snap.uptime).split(' ')[0], 10);
        if (Number.isFinite(s)) this.uptimeSecs = s;
      }

      // Nouvelles métriques
      if (snap.disks) this.sysDisks = snap.disks;
      if (snap.load_avg) this.sysLoadAvg = snap.load_avg;
      if (snap.cpu_temps) this.sysCPUTemps = snap.cpu_temps;
      if (snap.top_procs) this.sysTopProcs = snap.top_procs;
      if (snap.mem_cached_mb !== undefined) this.sysMemCachedMb = Number(snap.mem_cached_mb) || 0;
      if (snap.xen_info) this.xenInfo = snap.xen_info;
      if (snap.xen_domains) this.xenDomains = snap.xen_domains;

      // Bridges globaux
      if (typeof _updateNetGauge === 'function') _updateNetGauge(this, snap.net_rx_bps || 0, snap.net_tx_bps || 0);
      if (typeof pushSnapshot === 'function') pushSnapshot(snap);

      if (typeof renderNetworkMap === 'function' && snap.net_map) {
        const _role = this.hostRoleRole || 'Alpine';
        const _runtime = this.hostRoleRuntime || '';
        const _host = this.sysHostname || this.userName || 'host';
        const _ip = this.sysNetIp || '';
        window._nmLastArgs = [snap.net_map, _role, _runtime, _host, _ip];
        document.querySelectorAll('.netmap-container').forEach(el => {
          if (el.id) renderNetworkMap(snap.net_map, _role, _runtime, _host, _ip, el.id);
        });
      }

      _updatePerCoreGrid(snap.cpu_per_core, snap.cpu_freq_core);
      _updateNetIfacesGrid(snap.net_map);
      _updateDiskBars(snap.disks);
      _updateTempBars(snap.cpu_temps);
      _updateHtop(snap);

      this.$nextTick(() => _updateTopProcBars());
    },

    // ── Helpers internes ─────────────────────────────────────
    _setUser(data) {
      if (!data) return;
      this.userEmail = data.email || '';
      this.userRole = data.role || 'viewer';
      this.userDisplayName = data.display_name || '';
      this.userAvatar = data.avatar || '';
      this.userSSHKey = data.ssh_key || '';
      this.userPhotoUrl = data.photo_url || '';
      this.userDefaultCredentials = data.default_credentials === true;
      this.mustChangePassword = data.must_change === true;
      this._setUserName(data.display_name || data.email || '');
    },

    _setUserName(nameOrEmail) {
      const name = (nameOrEmail || '').split('@')[0] || 'admin';
      this.userName = name;
      this.userInitial = name.charAt(0).toUpperCase() || 'A';
    },

    _applySys(d) {
      if (!d) return;
      const s = (v, fb) => (v !== undefined && v !== null && v !== '') ? v : fb;
      const n = (v) => Number.isFinite(Number(v)) ? Number(v) : 0;

      // Boucle optimisée sur SYS_STATE_MAP
      for (const [srcKey, destKey] of Object.entries(SYS_STATE_MAP)) {
        if (d[srcKey] !== undefined) {
          this[destKey] = (srcKey.includes('_mb') || srcKey.includes('_bytes') || srcKey.includes('_bps') || srcKey.includes('cores') || srcKey.includes('usage'))
            ? n(d[srcKey])
            : s(d[srcKey], destKey.startsWith('sysNet') ? '—' : (destKey === 'sysOsRelease' ? 'Alpine Linux' : '—'));
        }
      }

      // Calculs dérivés
      const usage = this.sysCpuUsage;
      this.cpuPct = Math.round(Math.max(0, Math.min(100, usage)));
      const mt = this.sysMemTotalMb;
      const mu = this.sysMemUsedMb;
      this.memPct = mt > 0 ? Math.round((mu / mt) * 100) : 0;

      this._updateCSS();
    },

    _applyHostRole(hostRole, xenRoleLegacy) {
      if (hostRole && typeof hostRole === 'object') {
        const { role = '', runtime = '', label = role || '—', verified = false } = hostRole;
        if (role) {
          this.hostRoleRole = role;
          this.hostRoleRuntime = runtime;
          this.hostRoleLabelStr = label;
          this.hostRoleVerified = verified;
          this._applyRoleColor(role);
          return;
        }
      }
      if (xenRoleLegacy) {
        this.hostRoleRole = xenRoleLegacy;
        this.hostRoleRuntime = 'xen';
        this.hostRoleLabelStr = this._xenRoleLabel(xenRoleLegacy);
        this.hostRoleVerified = true;
        this._applyRoleColor(xenRoleLegacy);
        return;
      }
      this.hostRoleRole = 'Alpine';
      this.hostRoleRuntime = 'native';
      this.hostRoleLabelStr = 'Alpine Linux';
      this.hostRoleVerified = false;
      this._applyRoleColor('Alpine');
    },

    _applyRoleColor(role) {
      const roleKey = (String(role || '').trim().toLowerCase()) || 'alpine';
      const map = { dom0: 'dom0', domu: 'domu', container: 'container', alpine: 'alpine' };
      document.documentElement.setAttribute('data-role', map[roleKey] || 'alpine');
      this._updateThemeMetaColor(role || 'Dom0');
      if (window && window.__unyportSyncThemeColor) {
        window.__unyportSyncThemeColor(this.theme, role || 'Dom0');
      }
    },

    _themeColorByRole(theme, role) {
      const key = String(role || '').trim().toLowerCase();
      const light = {
        dom0: '#3e3aab',
        domu: '#a0284a',
        container: '#b05a1a',
        alpine: '#28587c',
      };
      const dark = {
        dom0: '#6864d4',
        domu: '#d45c82',
        container: '#de8d50',
        alpine: '#4d8cb0',
      };
      const palette = (theme === 'dark') ? dark : light;
      return palette[key] || palette.dom0;
    },

    _updateThemeMetaColor(role) {
      const currentTheme = (this.theme || document.documentElement.getAttribute('data-theme') || 'light');
      const meta = document.querySelector('meta[name="theme-color"]');
      if (meta) {
        meta.setAttribute('content', this._themeColorByRole(currentTheme, role || this.hostRoleRole || 'Dom0'));
      }
    },

    _setLoginThemeMetaColor() {
      const meta = document.querySelector('meta[name="theme-color"]');
      if (meta) meta.setAttribute('content', '#3e3aab');
    },

    _updateCSS() {
      const r = document.documentElement;
      // Optimisation: Ne pas écrire dans le DOM si la valeur n'a pas changé
      const cpuStr = `${this.cpuPct}%`;
      if (r.style.getPropertyValue('--cpu-pct') !== cpuStr) r.style.setProperty('--cpu-pct', cpuStr);

      const memStr = `${this.memPct}%`;
      if (r.style.getPropertyValue('--mem-pct') !== memStr) r.style.setProperty('--mem-pct', memStr);

      const cpuCol = this.cpuPct > 80 ? '#dc2626' : this.cpuPct > 60 ? '#d97706' : '#3e3aab';
      if (r.style.getPropertyValue('--cpu-color') !== cpuCol) r.style.setProperty('--cpu-color', cpuCol);

      const memCol = this.memPct > 85 ? '#dc2626' : this.memPct > 65 ? '#d97706' : '#16a34a';
      if (r.style.getPropertyValue('--mem-color') !== memCol) r.style.setProperty('--mem-color', memCol);
    },

    _updateLBU(data) {
      if (!data) return;
      const lbu = data.lbu;
      if (lbu && typeof lbu === 'object') {
        this.lbuPresent = lbu.present === true;
        this.lbuState = lbu.state || 'absent';
        this.lbuArchive = lbu.archive || '';
        return;
      }
      if (typeof data.lbu_dirty !== 'undefined') {
        this.lbuPresent = true;
        this.lbuState = data.lbu_dirty === true ? 'dirty' : 'clean';
      }
    },

    _formatUptime(str) {
      const secs = parseInt(String(str || '').split(' ')[0], 10);
      if (!Number.isFinite(secs)) return '—';
      let r = secs;
      const d = Math.floor(r / 86400); r %= 86400;
      const h = Math.floor(r / 3600); r %= 3600;
      const m = Math.floor(r / 60);
      const s = r % 60;
      const parts = [];
      if (d) parts.push(`${d}d`);
      if (h) parts.push(`${h}h`);
      if (m) parts.push(`${m}m`);
      if (s || !parts.length) parts.push(`${s}s`);
      return parts.join(' ');
    },

    _startUptimeTicker(rawUptime) {
      const secs = parseInt(String(rawUptime || '').split(' ')[0], 10);
      if (!Number.isFinite(secs)) return;
      this.uptimeSecs = secs;
      this.uptime = this._formatUptime(String(this.uptimeSecs));
      if (this._uptimeTimer) clearInterval(this._uptimeTimer);
      this._uptimeTimer = setInterval(() => {
        this.uptimeSecs += 1;
        this.uptime = this._formatUptime(String(this.uptimeSecs));
      }, 1000);
    },

    _resetState() {
      this._stopSSE();
      document.documentElement.removeAttribute('data-role');
      this.sysHostname = '—';
      this.sysOsRelease = 'Alpine Linux';
      this.sysOsVersion = '';
      this.sysKernel = '—';
      this.alpineLatestVer = '';
      this.alpineUpdateLevel = '';
      this.kernelLatestVer = '';
      this.kernelUpdateLevel = '';
      this.sysDate = '—';
      this.sysCpuModel = '—';
      this.sysCpuVendor = '—';
      this.sysCpuCores = 0;
      this.sysCpuUsage = 0;
      this.sysCpuFreqAvg = 0;
      this.sysCpuFreqMax = 0;
      this.sysMemTotalMb = 0;
      this.sysMemUsedMb = 0;
      this.sysMemFreeMb = 0;
      this.sysNetIface = '—';
      this.sysNetIp = '—';
      this.sysNetRxBytes = 0;
      this.sysNetTxBytes = 0;
      this.sysNetRxBps = 0;
      this.sysNetTxBps = 0;
      this.hostRoleRole = '';
      this.hostRoleRuntime = '';
      this.hostRoleLabelStr = '—';
      this.hostRoleVerified = false;
      this.apps = [];
      this.vms = [];
      this.uptime = '—';
      this.uptimeSecs = 0;
      if (this._uptimeTimer) { clearInterval(this._uptimeTimer); this._uptimeTimer = null; }
      this.cpuPct = 0;
      this.memPct = 0;
      this.lbuPresent = false;
      this.lbuState = 'absent';
      this.lbuArchive = '';
      this.sysDisks = [];
      this.sysLoadAvg = { load1: 0, load5: 0, load15: 0 };
      this.sysCPUTemps = [];
      this.sysTopProcs = [];
      this.sysMemCachedMb = 0;
      this.sysBoardName = ''; this.sysBoardVendor = ''; this.boardVersion = '';
      this.biosInfo = { bios_vendor: '', bios_version: '', bios_date: '', bios_year: '' }; this.biosAvailable = false;
      this.gpuList = []; this.gpuCount = 0;
      this.apkList = []; this.apkCount = 0; this.apkDbExists = false;
      this.modList = []; this.modCount = 0;
      this.svcList = []; this.svcStats = { started: 0, stopped: 0, crashed: 0, inactive: 0 }; this.svcFilter = 'all';
      this.logFiles = []; this.logLines = []; this.logSelectedFile = '';
      this.securitySummary = { ok: 0, warn: 0, critical: 0, unknown: 0 };
      this.securityChecks = []; this.securityServices = []; this.securityProcesses = []; this.securityListeners = [];
      this.securityError = ''; this.securityLoading = false;
      this.rebootHeatmapYear = new Date().getUTCFullYear();
      this.rebootHeatmapLeadBlank = 0;
      this.rebootHeatmapDays = [];
      this.rebootHeatmapLoading = false;
      this.rebootHeatmapError = '';
      this.rebootHeatmapTotal = 0;
      this.rebootHeatmapMaxPerDay = 0;
      this.rebootHeatmapDetailOpen = false;
      this.rebootHeatmapDetail = null;
      this.sseConnected = false;
      this.netRxVal = '0 B/s';
      this.netTxVal = '0 B/s';
      this.xenDomains = [];
      this.xenInfo = {};
      this.mustChangePassword = false;
      this.mustChangeOld = this.mustChangeNew = this.mustChangeConfirm = '';
      this.mustChangeError = '';
      this.userEmail = '';
      this.profileEmail = '';
      this.userRole = '';
      this.userDisplayName = '';
      this.userAvatar = '';
      this.userPhotoUrl = '';
      this.userSSHKey = '';
      this.userDefaultCredentials = false;
      this.email = '';
      this.password = '';
      this.loginPasswordVisible = false;
      this.profileOpen = false;
      this.dropdownOpen = false;
      this.currentPage = 'dashboard';
      this.brandingLogoSrc = '';
      this.brandingLogoPreview = '';
      this.adminUsersOpen = false;
    },

    _showLogin() {
      document.documentElement.setAttribute('data-auth-page', 'login');
      this._stopSSE();
      this._lockShellScroll(true);
      this._setLoginThemeMetaColor();
      this.page = 'login';
      this.mobileMenuOpen = false;
      if (window.location.pathname !== '/') {
        history.replaceState({}, '', '/');
      }
    },
    _lockShellScroll(locked) {
      const shell = document.getElementById('app-shell');
      const mainScroll = document.querySelector('.main-scroll');
      if (mainScroll) {
        if (locked) mainScroll.scrollTop = 0;
        mainScroll.style.overflowY = locked ? 'hidden' : 'auto';
        mainScroll.style.touchAction = locked ? 'none' : '';
      }
      if (shell) {
        shell.style.pointerEvents = locked ? 'none' : '';
      }
    },
  }));
});
