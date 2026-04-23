# `q.empjs.dev` 部署说明

这份文档对应 `0.3.x` 的最小上传授权流程。

目标结果：

- 首页管理入口：`https://q.empjs.dev/`
- 健康检查：`https://q.empjs.dev/v1/health`
- 上传链接：`https://q.empjs.dev/u/<grant_code>`
- 文件访问：`https://q.empjs.dev/files/...`

## 1. 下载二进制

以 `v0.3.0` 为例，Linux x86_64：

```bash
mkdir -p /opt/deployctl
cd /opt/deployctl

curl -L -o deployctl_linux_amd64.tar.gz \
  https://github.com/ckken/deployctl/releases/download/v0.3.0/deployctl_linux_amd64.tar.gz

tar -xzf deployctl_linux_amd64.tar.gz
mv deployctl-linux-amd64 deployctl
mv deployd-linux-amd64 deployd
chmod +x deployctl deployd
mkdir -p data
```

解压后应该至少有：

- `/opt/deployctl/deployctl`
- `/opt/deployctl/deployd`
- `/opt/deployctl/website/`

## 2. 启动 `deployd`

先前台验证：

```bash
cd /opt/deployctl

./deployd serve \
  --listen 127.0.0.1:7319 \
  --data-dir /opt/deployctl/data \
  --admin-secret '<你的adminKey>' \
  --web-dir /opt/deployctl/website
```

验证：

```bash
curl http://127.0.0.1:7319/v1/health
curl -I http://127.0.0.1:7319/
```

预期：

- `/v1/health` 返回 `{"status":"ok"}`
- 首页返回 `200`

## 3. 反代到 `q.empjs.dev`

Nginx 示例：

```nginx
server {
  listen 80;
  server_name q.empjs.dev;

  location / {
    proxy_pass http://127.0.0.1:7319;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
  }
}
```

然后：

```bash
nginx -t
systemctl reload nginx
```

如果是 Cloudflare、Tunnel、Caddy 或其他入口层，也一样：最终把 `q.empjs.dev` 指到 `127.0.0.1:7319`。

## 4. systemd 常驻

`/etc/systemd/system/deployd.service`

```ini
[Unit]
Description=deployctl deployd
After=network.target

[Service]
Type=simple
WorkingDirectory=/opt/deployctl
ExecStart=/opt/deployctl/deployd serve --listen 127.0.0.1:7319 --data-dir /opt/deployctl/data --admin-secret <你的adminKey> --web-dir /opt/deployctl/website
Restart=always
RestartSec=3
User=root

[Install]
WantedBy=multi-user.target
```

执行：

```bash
systemctl daemon-reload
systemctl enable --now deployd
systemctl status deployd --no-pager
```

## 5. 生成上传链接

命令行：

```bash
/opt/deployctl/deployctl --json --server https://q.empjs.dev upload-link create \
  --admin-key '<你的adminKey>' \
  --folder releases/demo \
  --expires-in 24h
```

返回结果里最重要的是：

- `upload_url`
- `folder`
- `expires_at`

例如：

```text
https://q.empjs.dev/u/upc_xxx
```

把这条链接发给目标 agent 即可。

## 6. 目标 agent 上传文件

`deployctl` 方式：

```bash
deployctl --json upload \
  --url https://q.empjs.dev/u/upc_xxx \
  --file ./build.zip
```

`curl` 方式：

```bash
curl -F file=@./build.zip https://q.empjs.dev/u/upc_xxx
```

返回结果会包含：

- `file_url`
- `saved_path`
- `uploaded_at`

## 7. 首页使用方式

首页地址：

- `https://q.empjs.dev/`

这页现在只做三件事：

1. 输入一次 `adminKey`
2. 点击 `生成并复制上传链接`
3. 查看最近上传链接和最近上传结果

高级参数折叠在 `<details>` 中：

- `folder`
- `expires_in`
- `max_files`

## 8. 验收清单

至少确认这几件事：

1. `curl https://q.empjs.dev/v1/health` 返回 `{"status":"ok"}`
2. `curl -I https://q.empjs.dev/` 返回 `200`
3. `deployctl upload-link create` 能返回 `https://q.empjs.dev/u/<grant_code>`
4. 目标 agent 用这条 URL 能成功上传文件
5. 返回值里能拿到 `file_url` 和 `saved_path`
