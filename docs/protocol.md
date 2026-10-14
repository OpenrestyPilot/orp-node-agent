# Agent protocol (v1)

Backend implementation status: the heartbeat, next-task, and result routes are now available on the dedicated optional mTLS listener configured by `ORP_AGENT_HTTPS_ADDR`. The browser/control-plane API port is separate. An administrator must bind each node's `agentId` and leaf-certificate SHA-256 fingerprint through `PUT /api/orp/nodes/{id}/agent/certificate` before the Agent can authenticate. The current task API supports only manually queued `reload` operations; it is not yet connected to candidate validation or publish batches.

The fingerprint is 64 lowercase or uppercase hexadecimal characters with no colons (`SHA256(cert.Raw)`). Send `{ "nodeId": "edge-01", "certificateFingerprint": "..." }` to register it, or `{ "nodeId": "edge-01", "revoke": true }` to revoke the bound certificate and expire outstanding tasks. Registration and task creation require a super-administrator Bearer token on the regular control-plane listener. `POST /api/orp/nodes/{id}/agent/reload` queues a fixed two-minute reload task; `GET /api/orp/nodes/{id}/agent/tasks` shows recent task outcomes.

The OpenResty Pilot node page also supports token-based quick enrollment. It issues a random node-specific Agent token, stores only its SHA-256 digest, and displays curl/wget or Docker startup commands once. Token-authenticated Agent requests use `Authorization: Bearer <token>` over the dedicated HTTPS listener. Rotating the token invalidates the previous one. Agent configs may set `agentToken` and omit client certificate/key files; `serverCAFile` remains required. Existing mTLS configs remain supported.

The Agent initiates every connection. All requests use HTTPS with a node-specific client certificate; the server certificate is checked against `serverCAFile`. The control plane must bind the certificate identity to the same registered `nodeId` in the request. No bearer secret or command is accepted from a task.

Base URL is the configured `controlPlaneUrl` and must address the Agent mTLS listener. The control-plane server certificate is validated by the Agent against `serverCAFile`; client certificates must chain to `ORP_AGENT_CLIENT_CA_FILE` and match the fingerprint registered for that node.

## Heartbeat

`POST /api/agent/v1/heartbeat`

```json
{
  "nodeId": "node-uuid",
  "state": {
    "collectedAt": "2026-10-08T12:00:00Z",
    "hostname": "edge-01",
    "os": "linux",
    "arch": "amd64",
    "openrestyVersion": "openresty/1.27.1.2",
    "openrestyRunning": true,
    "openrestyPid": 1234,
    "configReadable": true,
    "loadAverage1m": 0.15,
    "memoryTotalBytes": 8000000000,
    "memoryAvailableBytes": 4000000000
  }
}
```

Return any `2xx` with an empty body. The control plane must reject a node ID that does not match the authenticated client certificate.

## Poll one task

`GET /api/agent/v1/tasks/next?nodeId=node-uuid`

Return `204 No Content` when there is no task, or `200` and a task:

```json
{
  "id": "task-uuid",
  "nodeId": "node-uuid",
  "operation": "reload",
  "issuedAt": "2026-10-08T12:00:00Z",
  "expiresAt": "2026-10-08T12:02:00Z"
}
```

The backend must lease a task to one node, return an immutable operation and expiry, and avoid sending completed tasks again. The Agent accepts only `reload`; it cannot receive shell strings, command arguments, binary paths, configuration paths, or systemd unit names.

## Report result

`POST /api/agent/v1/tasks/{taskId}/result`

```json
{
  "taskId": "task-uuid",
  "nodeId": "node-uuid",
  "status": "succeeded",
  "startedAt": "2026-10-08T12:00:10Z",
  "completedAt": "2026-10-08T12:00:11Z",
  "output": "nginx: configuration file ... test is successful"
}
```

Statuses are `succeeded`, `failed`, or `rejected`. The backend must verify certificate identity, task ownership, task ID, and idempotency before recording the result. `output` is truncated to 4 KiB by the Agent.

## Security and delivery semantics

- Use a unique client certificate per node. Revoke it to disable the Agent.
- Restrict the private key to the Agent service account; never put it in this repository.
- Set a short task expiry and persist task state and results in the control plane.
- HTTPS client requests have a fixed timeout and response bodies are size-limited.
- Without a helper, the Agent runs `nginx -t -c <configured-path>` before `nginx -s reload -c <configured-path>` using its service account.
- When `reloadHelper` is configured, the Agent invokes only `/usr/bin/sudo -n <configured-helper>` with no task-supplied arguments. The root-owned helper validates and reloads fixed paths; allow exactly that helper in sudoers.
- The Agent durably caches completed results under `stateDir`; if a task is redelivered after result delivery failed, it resends the saved result rather than running reload again. The backend must still make result writes idempotent by task ID. A process crash between the local reload and caching its result remains an uncertain outcome and must be reconciled by the control plane before reissuing the operation.
