# UnyPort v0.60.3

This release fixes network topology reporting on Xen bridge hosts.

- Selects the IPv4-bearing bridge as the primary host interface.
- Reports bridge membership and the bridge address for physical and Xen VIF layer-2 ports.
- Distinguishes layer-2 ports without host addresses from missing IP data.
- Adds `UNYPORT_PORT` for native deployments that need a port other than 8800.
