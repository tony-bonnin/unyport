<div align="right">

<img width="5%" alt="logo-unyport-icon" src="git_assets/img/logo.jpg" />

</div>

<br><br>

<div align="center">

<img width="100%" alt="banner-unyport" src="git_assets/img/unyport-git-banner.png" />

</div>

<br><br>

# UnyPort (Beta)

**Unified sysadmin portal in Go — for Alpine Linux & Xen**

`Go` `Single Binary` `Single Port` `Xen-aware` `Data Disk Mode` `OAuth`

**Open source forever. No premium roadmap.**  
UnyPort stays open source: no “pro” lock-in, no future premium tier.  
Paid services are limited to **support/integration/operations by TRINITY**.

<br><br>

<div align="center">

<img width="100%" alt="Login Screenshot" src="git_assets/img/login.png" />

</div>


→ **[demo.unyport.app](https://demo.unyport.app)**

Legacy URL note: the previous address **dashboard.trinity-net.com** is still active.

<br>

🟪 **What is UnyPort**

UnyPort is a real-time system administration portal built in Go.
Single binary. Single port. Zero runtime dependency.

Designed specifically for Alpine Linux running in Data Disk Mode on Xen Type-1 hypervisors.
Every metric is read directly from the kernel — no agent, no daemon, no bloat.

Built in pure Go for deterministic deployment and low operational overhead on constrained hosts.
The roadmap is intentionally split:

- **V1 (current): Monitoring-first control plane**  
  live host metrics, security posture, service/process visibility, and Xen-aware context.
- **V2: Native orchestration layer**  
  VM lifecycle and mobility workflows built on Data Disk Mode primitives (`xl`, `xl migrate`) rather than a XenAPI abstraction layer.

In that model, **Data Disk Mode + `xl migrate`** is treated as a practical next-gen control plane approach ("xenapi.ng"): simpler, auditable, and aligned with minimal Dom0 operations.

UnyPort is also the continuation of the Alpine ACF effort I contributed to for official Alpine Linux workflows.  
That project is archived on Codeberg: [codeberg.org/trinity-labs/official](https://codeberg.org/trinity-labs/official).

---

<br>

⬛ **Features**

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

⬛ **Architecture**

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

⬛ **Why not Cockpit · Netdata · Portainer**

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

⬛ **Installation**

This repository ships a Docker-based UnyPort setup plus the full source tree.

**Prerequisites**

- Docker Engine
- Docker Compose plugin (`docker compose`)
- A free TCP port `8800`
- Optional: Nginx or another reverse proxy for HTTPS

**1. Open this repository**

```sh
cd docker_unyport
```

**2. Review the exposed port**

The bundled [docker-compose.yml](./docker-compose.yml) now exposes UnyPort on all interfaces by default:

```yaml
ports:
  - "8800:8800"
```

You can keep this default, or restrict the bind manually:

- use `8800:8800` to listen on all interfaces
- use `127.0.0.1:8800:8800` if UnyPort sits only behind local Nginx
- use `YOUR-HOST-IP:8800:8800` if you want to bind a specific interface

Example:

```yaml
ports:
  - "127.0.0.1:8800:8800"
```

**3. Start the stack**

```sh
docker compose up -d
docker compose logs -f unyport
```

**4. Open UnyPort**

- direct access: `http://YOUR-HOST:8800`
- behind Nginx: proxy to `http://127.0.0.1:8800` or to the host bind address you configured

If you terminate TLS at Nginx, also set `security_extra.https: true` in [unyport/backend/settings/settings.yaml](./unyport/backend/settings/settings.yaml) so auth and CSRF cookies are emitted with the correct security flags.

**Bundled local credentials**

This repository currently ships with a pre-seeded local user in [unyport/backend/settings/users.json](./unyport/backend/settings/users.json):

```text
Email    : demo@unyport.app
Password : aUniC0rnForUnyPort!
Role     : viewer
```

Important:

- this account is for local evaluation only
- it is currently a `viewer`, not an `admin`
- change the password before exposing the instance
- do not keep these credentials on any Internet-facing deployment

**Fresh admin bootstrap (optional)**

If you want a clean first-run admin account instead of the bundled demo user:

1. Stop the stack.
2. Remove `unyport/backend/settings/users.json`.
3. Add `UNYPORT_ADMIN_PASSWORD` to the `unyport` service environment in [docker-compose.yml](./docker-compose.yml).
4. Start the stack again.

Example:

```yaml
environment:
  UNYPORT_ASSETS: /app/unyport/frontend/public
  UNYPORT_ADMIN_PASSWORD: "ChangeThisToALongRandomPassword"
```

The first boot will then seed:

```text
Email    : demo@unyport.app
Password : <your UNYPORT_ADMIN_PASSWORD value>
Role     : admin
```

**Notes**

- OAuth client IDs and secrets in [unyport/backend/settings/config.yaml](./unyport/backend/settings/config.yaml) are placeholders and must be replaced before use.
- The default app proxy points to `ttyd`. If no `ttyd` service exists on the same network, `/proxy/ttyd/` will stay unavailable until you adapt the backend app config.

---

<br>

⬛ **Live Demo**

```
Running on TRINITY infrastructure — Alpine Linux v3.23.4 · Xen Type-1 · Data Disk Mode
Host    :  TRINITY Dom0
Kernel  :  6.18.33-0-lts
Memory  :  103M / 210M
Uptime  :  6d 12m
```

**Public demo credentials**

```text
Email    : demo@unyport.app
Password : aUniC0rnForUnyPort!
```

**Also available:** a **VM-on-demand demo** on [trinity-net.com](https://trinity-net.com), including **French Alpine Linux support** context and workflows.

<br><br>

<div align="center">

<img width="100%" alt="Dashboard Screenshot" src="git_assets/img/dashboard.png" />

<br><br>

<img width="100%" alt="Dashboard Screenshot" src="git_assets/img/map.png" />

</div>

→ [demo.unyport.app](https://demo.unyport.app)

---

<br>

⬛ **Part of TRINITY Edge Network**

UnyPort is only a small part of the TRINITY stack.

TRINITY also covers the underlying platform:
- minimalist and resilient Xen hypervisor operations (Dom0/DomU)
- low-power hardware strategy and edge deployment constraints
- Alpine-based sovereign infrastructure components beyond UnyPort

→ [trinity-net.com](https://trinity-net.com)
→ [gitlab.alpinelinux.org/trinity-labs](https://gitlab.alpinelinux.org/trinity-labs)

<br>

<div align="center">

*Contributor @ Alpine Linux · Est. 2020 · Versailles, France*

**A system you understand is a system you control.**

<img src="git_assets/img/flag.png" width="2%">

</div>
