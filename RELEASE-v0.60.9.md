# UnyPort 0.60.9

- Refines the complete network-device icon set with consistent dual-tone rendering based on the existing UnyPort topology icons.
- Adds an explicit `device_type` to every ARP neighbor returned by the backend.
- Supports per-installation device inventories through `UNYPORT_DEVICE_TYPES` using validated `IP=type` entries.
- Detects common virtual-machine MAC prefixes as servers and safely falls back to `unknown` instead of returning an empty type.
- Adds automated coverage for explicit, virtualized, invalid and unidentified device types.
