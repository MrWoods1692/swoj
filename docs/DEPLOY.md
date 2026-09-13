# 部署指南

## 前置

- Linux x86_64（或其他受 Go 支持的平台）
- 1GB 以上空闲磁盘
- 512MB 以上内存
- 网络可访问 AI 提供商（如 yunzhiapi.cn）

## 1. 构建

```bash
# 后端
cd swoj && go build -o swoj ./cmd/swoj

# 前端
cd web && npm install && npm run build
cd ..
```

产物：

- `swoj`：后端二进制
- `web/dist/`：前端构建产物

## 2. 部署目录

```bash
sudo mkdir -p /opt/swoj
sudo cp swoj /opt/swoj/
sudo cp -r web/dist /opt/swoj/web/
sudo mkdir -p /opt/swoj/data /opt/swoj/logs
sudo useradd -r -s /sbin/nologin swoj 2>/dev/null || true
sudo chown -R swoj:swoj /opt/swoj
```

## 3. 配置 judge

测评内置执行器，默认即可使用，**无需安装 go-judge**。编译走本机 g++（C++17），
单次执行用 C 包装器施加 rlimit（CPU/内存/文件大小）。

```bash
g++ --version   # 需要 g++ 11+，编译与测评同一套工具链
```

`SWOJ_JUDGE_BIN` 仅用于接入外部测评程序，留空即使用内置执行器；
`/api/judge/info` 的 `builtin_judge` 字段为 `true` 表示当前走内置。

## 4. systemd 服务

`/etc/systemd/system/swoj.service`：

```ini
[Unit]
Description=SWOJ Online Judge
After=network.target

[Service]
Type=simple
WorkingDirectory=/opt/swoj
User=swoj
Group=swoj
Environment=SWOJ_LISTEN=127.0.0.1:8080
Environment=SWOJ_STATIC=/opt/swoj/web/dist
Environment=SWOJ_DATA_DIR=/opt/swoj/data
Environment=SWOJ_HOST=https://oj.example.com
Environment=SWOJ_SECRET=<随机 32 位 JWT 密钥>
Environment=SWOJ_OAUTH_SECRET=<Campux 应用 Secret>
Environment=SWOJ_OAUTH_SCOPE=profile
Environment=SWOJ_JUDGE_WORKERS=4
Environment=SWOJ_JUDGE_QUEUE=64
Environment=SWOJ_MEM_LIMIT=256
Environment=SWOJ_AI_TOKEN=<可选：AI 提供商 Token>
ExecStart=/opt/swoj/swoj
ExecReload=/bin/kill -HUP $MAINPID
Restart=always
RestartSec=3
StandardOutput=append:/opt/swoj/logs/swoj.log
StandardError=append:/opt/swoj/logs/swoj.log
ProtectSystem=strict
PrivateTmp=true
NoNewPrivileges=true
ReadWritePaths=/opt/swoj/data

[Install]
WantedBy=multi-user.target
```

启动：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now swoj
sudo systemctl status swoj
```

## 5. nginx 反向代理

`/etc/nginx/conf.d/oj.conf`：

```nginx
upstream swoj {
    server 127.0.0.1:8080;
    keepalive 32;
}

server {
    listen 443 ssl http2;
    server_name oj.example.com;

    ssl_certificate /etc/letsencrypt/live/oj.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/oj.example.com/privkey.pem;

    client_max_body_size 20m;    # AI 文件上传上限 3MB

    location / {
        proxy_pass http://swoj;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_read_timeout 120s;   # AI 问答可能较慢
        proxy_send_timeout 60s;
    }

    # 前端静态资源（可选，让 nginx 直接 serve 更快）
    location /assets/ {
        alias /opt/swoj/web/dist/assets/;
        expires 30d;
        add_header Cache-Control "public, immutable";
    }
}

server {
    listen 80;
    server_name oj.example.com;
    return 301 https://$host$request_uri;
}
```

## 6. Campux OAuth 应用注册

登录前必须在 Campux 管理后台登记应用，否则授权页会提示
「redirect_uri 未在应用中注册」。

| 项 | 值 |
|---|---|
| 应用 ID | `4ROQNWLOP5zRkhQe`（`SWOJ_OAUTH_CLIENT_ID`，代码内置默认值） |
| 应用 Secret | Campux 后台生成（可轮换），服务端填入 `SWOJ_OAUTH_SECRET` |
| 回调地址 | `{SWOJ_HOST}/auth/campux/callback` |
| 授权类型 | authorization_code |
| PKCE | S256（`enable_pkce`） |
| Scopes | `profile` |

### 登记步骤

1. 用有管理员权限的账号登录 Campux，进管理后台的 OAuth 应用管理页。
2. 若应用不存在，点「新建 OAuth 应用」；应用 ID 会由 Campux 生成，
   填好后把生成的 ID 写进 `SWOJ_OAUTH_CLIENT_ID`。
3. 在「回调地址，每行一个」里加一行，**一行只能一个地址**：

   ```
   http://localhost:8080/auth/campux/callback
   ```

4. 在「允许的权限范围」里加一行：

   ```
   profile
   ```

5. 勾选启用与「需要 PKCE」，保存。
6. 点该应用的「轮换 Secret」，把新生成的值填进服务端
   `SWOJ_OAUTH_SECRET`，重启服务。

注意：白名单按精确字符串比对，协议、域名、端口、路径任一不同都算未注册；
地址里**不要带末尾斜杠**。授权页需要先登录 Campux 账号才能点「同意授权」，
未登录时授权接口直接返回 `401 请先登录`。

回调地址的协议、域名、路径必须与 `SWOJ_HOST` 完全一致。
生产环境 `SWOJ_HOST=https://oj.example.com` 时，登记
`https://oj.example.com/auth/campux/callback`。

代码实际发出的授权请求（可用 curl 自查）：

```bash
curl -s -c /tmp/c.txt -o /dev/null -w '%{redirect_url}\n' \
  http://127.0.0.1:8080/auth/campux | python3 -c "import sys,urllib.parse;print(urllib.parse.unquote(sys.stdin.read()))"
```

只申请 `profile`：`/oauth/userinfo` 只读取 `name` 与 `username`（QQ 号），
未使用 `tenant` scope。若确需租户信息，先在代码里消费该字段再放宽
`SWOJ_OAUTH_SCOPE`。

Mock 模式（`SWOJ_OAUTH_MOCK=1`）跳过真实回调，但 `OAuthEnabled()` 仍要求
`SWOJ_OAUTH_SECRET` 非空；本地联调填任意占位值即可，
不要把它填成真实 Secret。

Campux 对 `/oauth/*` 全部 308 强制跳转 https。代码默认
`SWOJ_OAUTH_BASE` 已是 `https://kg.campux.top`；token 交换用的
`http.Client` 会跟随 308 并在 POST 上保留方法，因此即使误配成 http
也只是多跳一次，不会导致授权失败。

## 7. 首次启动

```bash
sudo systemctl start swoj
journalctl -u swoj -f
```

浏览器打开 https://oj.example.com → 完成校园墙登录 → 首位用户成为 `super`。

进入「管理后台 → AI 配置」填入 yunzhiapi.cn Token 即可启用 AI 问答。

## 8. 备份与恢复

SQLite 数据库文件：`/opt/swoj/data/swoj.db`。

在线备份（推荐）：

```bash
sqlite3 /opt/swoj/data/swoj.db ".backup '/opt/swoj/backups/swoj-$(date +%F).db'"
```

恢复：

```bash
sudo systemctl stop swoj
sudo cp /opt/swoj/backups/swoj-2026-09-13.db /opt/swoj/data/swoj.db
sudo chown swoj:swoj /opt/swoj/data/swoj.db
sudo systemctl start swoj
```

## 9. 升级

```bash
git pull origin main
sudo systemctl stop swoj
cd swoj
go build -o swoj ./cmd/swoj
cp swoj /opt/swoj/swoj
cd web && npm install && npm run build
cp -r dist /opt/swoj/web/
cd ..
sudo systemctl start swoj
```

**注意**：schema 变更直接改 `CREATE TABLE`（项目未部署无历史库，见 `schema_core.go:9` 注释）。若你的部署有历史数据，需要在部署前先手动迁移。

## 10. 健康检查

```bash
curl -s http://127.0.0.1:8080/api/status      | jq   # 服务总览与队列
curl -s http://127.0.0.1:8080/api/judge/info  | jq   # 测评执行器信息
```

没有 `/api/health`；存活检查用 `/api/status`。

## 11. 常见问题

**Q: 服务启动失败？**
A: 检查 `journalctl -u swoj -n 100`；常见原因：端口占用、`SWOJ_STATIC` 路径不存在、SQLite 文件权限错误。

**Q: AI 问答返回 503？**
A: 管理员未配置 Token，或未启用 AI 服务。进入管理后台 → AI 配置。

**Q: 提交返回 503？**
A: 测评队列已满或入队失败（查看 `journalctl -u swoj -n 100`）。与 `SWOJ_JUDGE_BIN` 无关——
测评为内置执行器，该变量留空即使用内置；只有队列满才返回 503。

**Q: 校园墙登录返回 503？**
A: 未配置 `SWOJ_OAUTH_SECRET`。该变量必须与 Campux 应用 Secret 一致，
留空或填占位符都会导致 `校园墙登录未配置`。

**Q: 授权页提示 redirect_uri 未注册？**
A: Campux 应用里登记的回调地址必须与 `SWOJ_HOST` 拼出的地址完全一致：
`{SWOJ_HOST}/auth/campux/callback`，无末尾斜杠。生产环境把 `SWOJ_HOST`
设为 `https://oj.example.com` 后，登记的地址就是
`https://oj.example.com/auth/campux/callback`。

**Q: 内存占用过高？**
A: 调低 `SWOJ_JUDGE_WORKERS`（默认 2），或调低 `SWOJ_MEM_LIMIT`（默认 256 MB）。

**Q: 登录时提示「未找到授权凭证」或「授权状态已过期」？**
A: 两条都出自回调的 state 校验，含义不同：

- **未找到授权凭证**：回调请求里没带上 `swoj_oauth_state` cookie。常见两种原因——
  一是授权入口与回跳地址的 Host 不一致（例如入口用 `localhost`、
  Campux 回跳到 `127.0.0.1`，两者是两个不同的 cookie 域），
  二是直接打开了回调 URL 而没有从 `/auth/campux` 入口开始。
  统一成同一个访问地址即可，`SWOJ_HOST` 必须与实际访问地址一致。
- **授权状态已过期或与当前页面不一致**：cookie 在但 state 对不上，
  一般是授权凭证已过期（有效期 10 分钟）或复用了旧的授权链接，
  刷新页面重新点一次登录即可。

**Q: 如何查看日志？**
A: `journalctl -u swoj -f`，或访问 `/admin/logs`。
