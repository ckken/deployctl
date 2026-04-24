# deployctl

`deployctl` 现在的主目标是一个最小可用的上传授权服务。

你先把 `deployd` 部署到自己的域名上，例如 `https://q.empjs.dev`。  
管理侧 agent 用 `adminKey` 生成一条上传链接，再把这条链接发给另一个 agent；对方 agent 直接按 URL 上传文件，不需要再领 token，也不需要再录入目录和时间。

当前版本是 `0.3.x` 基线，核心能力只有三件事：

- 管理员生成上传链接
- 目标 agent 通过上传链接直接传文件
- 服务记录文件地址、保存目录和上传时间

## 主流程

1. 启动 `deployd`
2. 管理 agent 生成上传链接
3. 把返回的 URL 发给目标 agent
4. 目标 agent 上传文件
5. 服务返回 `file_url` 和 `saved_path`

## 命令行

生成上传链接：

```bash
deployctl --json --server https://q.empjs.dev upload-link create \
  --admin-key '<adminKey>' \
  --folder releases/demo \
  --expires-in 24h
```

返回结果示例：

```json
{
  "grant_id": "grt_123",
  "grant_code": "upc_456",
  "upload_url": "https://q.empjs.dev/u/upc_456",
  "folder": "releases/demo",
  "max_files": 1,
  "created_at": "2026-04-24T12:00:00Z",
  "expires_at": "2026-04-25T12:00:00Z",
  "upload_path": "/releases/demo"
}
```

查看上传链接状态：

```bash
deployctl --json upload --url https://q.empjs.dev/u/upc_456
```

上传文件：

```bash
deployctl --json upload \
  --url https://q.empjs.dev/u/upc_456 \
  --file ./build.zip
```

也可以直接用 `curl`：

```bash
curl -F file=@./build.zip https://q.empjs.dev/u/upc_456
```

## 默认值

- `folder`: `uploads/YYYY/MM/DD`
- `expires_in`: `24h`
- `max_files`: `1`

你也可以在创建上传链接时显式指定：

- `--folder releases/demo`
- `--expires-in 72h`
- `--max-files 3`

## 仍然保留的基础能力

历史上的 token-only 认证底座还在，方便后续继续扩展：

- `deployctl --json doctor`
- `deployctl --json auth whoami`
- `deployctl auth token set <token>`
- `deployd admin create-token`

但它们不再是上传主流程。

## 本地运行

启动服务端：

```bash
go run ./cmd/deployd serve \
  --listen :7319 \
  --data-dir ./.deployctl-data \
  --admin-secret dev-secret \
  --web-dir ./website
```

创建上传链接：

```bash
go run ./cmd/deployctl --json --server http://127.0.0.1:7319 upload-link create \
  --admin-key dev-secret
```

## Dashboard

首页是一个同源托管的 dashboard，默认由 `deployd` 所在域名提供：

- 用 `adminKey` 登录，凭据只保存在当前浏览器
- 顶部总览可用链接、剩余上传次数、上传记录和最近活动
- 一键生成并复制上传链接
- 展示上传链接和最近上传结果
- 点击退出会清空本地 `adminKey` 并回到登录页

相关文件：

- `website/index.html`
- `website/site.css`
- `website/site.js`

## 服务器部署

`q.empjs.dev` 的部署说明见：

- `docs/deploy-q-empjs-dev.md`
