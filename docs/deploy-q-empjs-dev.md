# `q.empjs.dev` 部署说明

这份文档是给服务器 agent 直接执行的部署方案。目标是把：

- 首页管理台挂到 `https://q.empjs.dev/`
- share link 挂到 `https://q.empjs.dev/s/<share_code>`
- API 走同一个域名下的 `/v1/...`

## 目标结果

部署完成后应满足：

- `https://q.empjs.dev/` 打开就是管理首页
- `https://q.empjs.dev/v1/health` 返回 `200`
- 创建出来的分享地址格式是 `https://q.empjs.dev/s/<share_code>`

## 1. 下载二进制

以 `v0.2.2` 为例，Linux x86_64 服务器执行：

```bash
mkdir -p /opt/deployctl
cd /opt/deployctl

curl -L -o deployctl_linux_amd64.tar.gz \
  https://github.com/ckken/deployctl/releases/download/v0.2.2/deployctl_linux_amd64.tar.gz

tar -xzf deployctl_linux_amd64.tar.gz
mv deployctl-linux-amd64 deployctl
mv deployd-linux-amd64 deployd
chmod +x deployctl deployd
mkdir -p data
```

解压后目录里应该至少有：

- `./deployctl`
- `./deployd`
- `./website/`

## 2. 启动 `deployd`

先直接前台验证一遍：

```bash
cd /opt/deployctl

./deployd serve \
  --listen 127.0.0.1:7319 \
  --data-dir /opt/deployctl/data \
  --admin-secret '<你的adminKey>' \
  --web-dir /opt/deployctl/website
```

如果服务正常启动，保留这个进程，另开一个终端继续做反代验证。

## 3. 配置 Nginx 反代

在服务器上写入：

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

例如保存到：

- `/etc/nginx/conf.d/q.empjs.dev.conf`

然后执行：

```bash
nginx -t
systemctl reload nginx
```

如果你的 TLS 终止层不是 Nginx，而是 Cloudflare Tunnel、Caddy 或网关层，也一样要保证最终把 `q.empjs.dev` 转发到 `127.0.0.1:7319`。

## 4. 验证首页与 API

在服务器上执行：

```bash
curl -I http://127.0.0.1:7319/
curl -I http://127.0.0.1:7319/v1/health
curl -I https://q.empjs.dev/
curl -I https://q.empjs.dev/v1/health
```

预期：

- 本机 `127.0.0.1:7319` 返回 `200`
- 域名 `https://q.empjs.dev/` 返回 `200`
- 不是 Cloudflare 的 `404 page not found`

## 5. 配置成 systemd 常驻

确认前台运行没问题后，再改成 systemd：

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

例如保存到：

- `/etc/systemd/system/deployd.service`

然后执行：

```bash
systemctl daemon-reload
systemctl enable --now deployd
systemctl status deployd --no-pager
```

## 6. 首页使用方式

部署完成后：

1. 打开 `https://q.empjs.dev/`
2. 输入 `adminKey`
3. 创建用户 token 或 share link
4. 把 share link 发给 agent

最终发给 agent 的链接格式应该是：

```text
https://q.empjs.dev/s/<share_code>
```

不是：

```text
https://q.empjs.dev/?share=...&code=...
```

## 7. 常见错误

### 只看到 Cloudflare 404

说明 `q.empjs.dev` 还没有正确反代到 `deployd`，不是前端没更新。

优先检查：

- Nginx 是否加载了 `q.empjs.dev` 的配置
- 域名是否真的指向这台服务器
- Cloudflare 回源是否到了这台机器
- 本机 `127.0.0.1:7319` 是否已经返回 `200`

### 首页能打开，但没有管理功能

优先检查：

- 启动命令里是否带了 `--web-dir /opt/deployctl/website`
- 解压包里是否包含 `website/`
- 打开的是否真的是 `q.empjs.dev`，而不是 GitHub Pages 说明页

## 8. 最小验收清单

全部完成后，至少确认这 4 件事：

1. `curl -I https://q.empjs.dev/` 返回 `200`
2. `curl -I https://q.empjs.dev/v1/health` 返回 `200`
3. 首页输入 `adminKey` 后能创建 share link
4. share link 形如 `https://q.empjs.dev/s/<share_code>`
