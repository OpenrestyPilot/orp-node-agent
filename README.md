# ORP 节点 Agent

`orp-node-agent` 是一个独立的 Go 项目，用于将 OpenResty 实例注册到 ORP。

## 职责

- 向控制面报告节点身份、OpenResty 版本与构建信息、进程健康、主机资源，以及配置启用的本地状态指标。
- 接收并回报明确授权的固定节点操作结果。当前首个操作是针对一个指定 OpenResty 服务单元执行 `reload`。
- 主动与控制面建立经过身份验证的连接；不在受管节点上暴露未经身份验证的监听端口。

## 安全边界

- Agent 与控制面之间使用 HTTPS；可采用节点级 mTLS 证书或可轮换的 Bearer Token。
- 请求只能引用已登记的操作和不可变任务 ID，不能传入 Shell 命令、可执行文件路径、服务单元或任意参数。
- 本地配置固定 OpenResty 二进制文件、配置路径和可选的特权辅助程序。重载权限仅限于该固定目标。
- 节点 Control API 是独立能力，Agent 不会调用它；Agent 的重载操作使用自身受限的本地服务动作。
- 状态报告和任务结果可追溯到已注册节点，并由控制面留存审计。

## 目录结构

```text
cmd/orp-node-agent/          Agent 进程入口
internal/config/             本地配置校验
internal/collector/          OpenResty 与主机状态采集
internal/controlplane/       经身份验证的心跳与任务传输
internal/operations/         固定且经允许列表限制的本地操作
deploy/systemd/              受限权限的服务配置示例
```

## 项目状态

Agent 可执行程序及其主动建立的 HTTPS 协议客户端已经实现。OpenResty Pilot 控制面通过专用 HTTPS 监听器提供基于证书指纹或节点 Token 的心跳、固定 `reload` 任务领取，以及幂等的结果提交接口。管理员可以从节点实例页面签发 Token 并复制二进制或 Docker 接入命令；mTLS 节点仍可按证书指纹登记。

该能力尚未接入候选配置校验或发布批次，因此目前不能用于向外部节点发布配置。协议格式与集成边界见 [`docs/protocol.md`](docs/protocol.md)。

## 构建与运行

```sh
go test ./...
go build -o dist/orp-node-agent ./cmd/orp-node-agent
sudo install -m 0600 deploy/orp-node-agent.example.json /etc/orp-node-agent/config.json
sudo dist/orp-node-agent -config /etc/orp-node-agent/config.json
```

配置必须包含控制面 HTTPS 地址、节点身份和服务端 CA；选择 mTLS 时还需提供客户端证书和密钥，Token 模式则配置 `agentToken`。Agent 会拒绝非 HTTPS 地址，也不会从任务载荷接受命令、路径或服务名称。

如果 OpenResty 主进程由 root 运行，请将固定的配置校验与重载辅助程序以 root 所有、权限 `0755` 的方式安装到 `/usr/local/sbin/orp-openresty-reload`，再将 `deploy/systemd/orp-node-agent.sudoers` 安装为 `/etc/sudoers.d/orp-node-agent`（权限 `0440`，并用 `visudo` 校验）。Agent 只会通过 `/usr/bin/sudo -n` 调用不带参数的辅助程序；该程序以 root 权限运行 `nginx -t` 并重载固定配置。若节点使用不同路径，请先修改辅助程序中的 OpenResty 路径。Agent 账户不得修改辅助程序、OpenResty 二进制文件或配置目录。

示例 systemd 单元以 `orp-agent` 用户运行 Agent，并且只授予其通过 sudo 调用无参数辅助程序的权限。配置有意不启用 `NoNewPrivileges` 和挂载命名空间隔离，因为这些设置可能阻止上述严格受限的 sudo 提权；控制面请求仍无法指定辅助程序参数或操作目标。
