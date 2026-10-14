# ORP Node Agent

Project identifier: `orp-node-agent`.

The Agent is an independent Go project for enrolling external OpenResty instances into ORP.

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

The Agent executable supports outbound mTLS or Bearer-token authentication, state collection, fixed reload operations, and systemd deployment. OpenResty Pilot exposes node-token issuance, heartbeats, task queue, and result endpoints. Configuration artifact transfer and publish-batch integration remain incomplete; see `docs/protocol.md`.
