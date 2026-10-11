# ORP Node Agent

Project identifier: `orp-node-agent`.

The Agent is an independent Go project for enrolling external OpenResty instances into OpenResty Plus.

## Responsibilities

- Report node identity, OpenResty version/build information, process health, host resources, and configured local status metrics.
- Maintain an outbound authenticated connection to the control plane using mutual TLS and a revocable per-node identity.
- Execute only explicitly authorized fixed operations. The initial operation is reload of one locally configured OpenResty instance; requests cannot supply shell commands, executable paths, service units, or arbitrary arguments.
- Keep the node's Control API independent. The Agent does not call that API.

## Layout

```text
cmd/orp-node-agent/          Agent process entry point
internal/config/             Validated local configuration
internal/collector/          OpenResty and host state collection
internal/controlplane/       Authenticated heartbeat and task transport
internal/operations/         Fixed, allowlisted local operations
deploy/systemd/              Restricted service and reload helper examples
```

## Status

The Agent executable, state collector, outbound mTLS client, fixed reload operation, and systemd examples are implemented. The Go control plane does not yet expose Agent enrollment, heartbeat ingestion, task queue, or result endpoints. External-node publishing and production integration remain incomplete until those backend routes and certificate enrollment are implemented; see `docs/protocol.md`.
