<div align="center">
<img width="33%" alt="logo-unyport-icon" src="git_assets/img/logo.jpg" />
</div>

<br><br>

# UnyPort (Beta)

**Unified sysadmin portal in Go — for Alpine Linux & Xen**

`Go` `Single Binary` `Single Port` `Xen-aware` `Data Disk Mode` `OAuth`

<br><br>

<img width="2077" height="1797" alt="Capture d&#39;écran 2026-05-08 232427" src="git_assets/img/login.png" />


→ **[dashboard.trinity-net.com](https://dashboard.trinity-net.com)**

<br>

🟥 **What is UnyPort**

UnyPort is a real-time system administration portal built in Go.
Single binary. Single port. Zero runtime dependency.

Designed specifically for Alpine Linux running in Data Disk Mode on Xen Type-1 hypervisors.
Every metric is read directly from the kernel — no agent, no daemon, no bloat.

---

<br>

🟪 **Features**

| Module | Description |
|---|---|
| **System** | OS, kernel, uptime, board info — live |
| **CPU** | Per-core frequency, load, model |
| **Memory** | Used / total, real-time graph |
| **Network** | Interface, IP, RX/TX rates, totals |
| **Storage** | Disk I/O — sda, nvme read/write |
| **Processes** | Top processes by resource usage |
| **Security** | CSRF, JWT, CSP, TLS, 2FA, OAuth status |
| **Logs** | Recent system logs with severity |
| **Thermal** | CPU pkg, core avg, board, NVMe temps |
| **Apps** | Proxy status, active connections |
| **Xen** | Dom0/DomU detection, VM lifecycle |

---

<br>

🟦 **Architecture**

| Layer | Detail |
|---|---|
| **Runtime** | Single Go binary — musl compatible |
| **Transport** | Single port — HTTP/2 + WebSocket |
| **Auth** | OAuth GitHub · OAuth GitLab · JWT HS256 |
| **Security** | CSRF · CSP strict · TLS · 2FA ready |
| **Metrics** | Direct kernel reads — /proc · /sys |
| **State** | Stateless — compatible Data Disk Mode |
| **Hypervisor** | Xen Type-1 aware — Dom0 detection |

---

<br>

🟨 **Why not Cockpit · Netdata · Portainer**

| | UnyPort | Cockpit | Netdata | Portainer |
|---|:---:|:---:|:---:|:---:|
| **Single binary** | ✓ | ✗ | ✗ | ✗ |
| **No systemd** | ✓ | ✗ | ✗ | ✓ |
| **musl / Alpine native** | ✓ | ✗ | Partial | Partial |
| **Data Disk Mode** | ✓ | ✗ | ✗ | ✗ |
| **Xen-aware** | ✓ | ✗ | ✗ | ✗ |
| **Single port** | ✓ | ✗ | ✗ | ✗ |
| **Zero agent** | ✓ | ✗ | ✗ | ✓ |

---

<br>

🟫 **Quick Start**

```sh
# Clone
git clone https://github.com/tony-bonnin/unyport.git
cd unyport

# Run with Docker
docker compose up -d

# Or build natively on Alpine Linux
cd unyport
go build -o unyport .
./unyport
```

Access : `https://your-host:PORT`

---

<br>

🟥 **Live Demo**

```
Running on TRINITY infrastructure — Alpine Linux v3.23 · Xen Type-1 · Data Disk Mode
Host    :  TRINITY Dom0
Kernel  :  6.18.9-0-lts
Memory  :  103M / 210M
Uptime  :  6d 12m
```

<br><br>

<img width="1570" height="1241" alt="Capture d&#39;écran 2026-05-09 045725" src="git_assets/img/dashboard.png" />


→ [dashboard.trinity-net.com](https://dashboard.trinity-net.com)

---

<br>

⬛ **Part of TRINITY Edge Network**

UnyPort is the control plane of the TRINITY sovereign infrastructure stack.

→ [trinity-net.com](https://trinity-net.com)
→ [gitlab.alpinelinux.org/trinity-labs](https://gitlab.alpinelinux.org/trinity-labs)

<br>

<div align="center">

*Contributor @ Alpine Linux · Est. 2020 · Versailles, France*

**A system you understand is a system you control.**

<img src="git_assets/img/flag.png" width="2%">

</div>