# deployctl

`deployctl` 是一个面向 agent / CI 的 token-only 部署控制面起步仓库。当前版本只实现认证底座：

- `deployd`：本地 token 服务端
- `deployctl`：本地 CLI
- token 创建、吊销、校验、`whoami`
- CLI `doctor` 与本地 token 配置

首版不包含用户名密码、登录会话、refresh token，也不包含真正的上传部署逻辑。

## GitHub 二进制

发布页会提供以下二进制压缩包：

- `deployctl_linux_amd64.tar.gz`
- `deployctl_linux_arm64.tar.gz`
- `deployctl_darwin_arm64.tar.gz`

每个压缩包都包含：

- `deployctl`
- `deployd`

服务器侧可以直接下载并解压，例如：

```bash
curl -L -o deployctl_linux_amd64.tar.gz <release-asset-url>
tar -xzf deployctl_linux_amd64.tar.gz
chmod +x deployctl-linux-amd64 deployd-linux-amd64
mv deployctl-linux-amd64 /usr/local/bin/deployctl
mv deployd-linux-amd64 /usr/local/bin/deployd
```

如果需要保留原始二进制名，可以自行重命名：

```bash
mv deployctl-linux-amd64 deployctl
mv deployd-linux-amd64 deployd
```

## 本地运行

启动服务端：

```bash
go run ./cmd/deployd serve --listen :7319 --data-dir ./.deployctl-data --admin-secret dev-secret
```

创建 token：

```bash
go run ./cmd/deployd admin create-token \
  --data-dir ./.deployctl-data \
  --admin-secret dev-secret \
  --name ci-bot \
  --scope read-only
```

设置 CLI token：

```bash
go run ./cmd/deployctl auth token set <token>
```

检查状态：

```bash
go run ./cmd/deployctl --json doctor
go run ./cmd/deployctl --json auth whoami
```

## Token 来源优先级

1. `--token`
2. `DEPLOYCTL_TOKEN`
3. `~/.deployctl/config.toml`

## HTTP API

- `GET /v1/health`
- `GET /v1/auth/whoami`
- `POST /v1/admin/tokens`
- `GET /v1/admin/tokens`
- `POST /v1/admin/tokens/{token_id}/revoke`

`/v1/auth/whoami` 使用 `Authorization: Bearer <token>`。  
`/v1/admin/*` 使用 `X-Admin-Secret: <secret>`。
