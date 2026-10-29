# Changelog

All notable changes to this project will be documented in this file.

The format is based on Keep a Changelog, and this project follows Semantic Versioning.

## [0.60.1] - 2026-10-29

### Fixed
- Removed generated runtime logs and binaries from version control so service restarts cannot retrigger release automation.

## [0.60.0] - 2026-10-29

### Changed
- Made watcher retries idempotent; versions now change only during an explicit release.
- Updated the Go 1.27.3 toolchain and module dependencies.
- Bounded log-tail memory usage and added regression tests.
- Simplified and hardened the development Compose service.
- Removed stale hard-coded version text from the frontend title.

## [0.1.0] - 2026-06-07

### Added
- First public release of UnyPort.
- Real-time system administration dashboard built in Go.
- Alpine Linux and Xen oriented deployment model.
- Single binary, single port architecture.
- Monitoring modules for system, CPU, memory, network, storage, processes, logs, thermal data, apps, and Xen context.
- Authentication and security foundations including OAuth, JWT, CSRF, CSP, TLS, and 2FA-ready flows.
- Docker-based development workflow and native Go build path.

### Notes
- This release establishes the initial public baseline for UnyPort.
- The project remains in active evolution toward broader orchestration features.
