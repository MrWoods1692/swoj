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

安装 go-judge：

```bash
sudo apt install g++ cmake
git clone https://github.com/mrcj/GoJudge /tmp/gojudge && cd /tmp/gojudge
./install.sh   # 输出 /usr/local/bin/go-judge
```

验证：

```bash
go-judge --version
```

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
Environment=SWOJ_JUDGE_BIN=/usr/local/bin/go-judge
Environment=SWOJ_WORKERS=4
Environment=SWOJ_QUEUE_SIZE=64
Environment=SWOJ_MEM_LIMIT_MB=256
Environment=SWOJ_OAUTH_SECRET=<随机 32 位>
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

## 6. 首次启动

```bash
sudo systemctl start swoj
journalctl -u swoj -f
```

浏览器打开 https://oj.example.com → 完成校园墙登录 → 首位用户成为 `super`。

进入「管理后台 → AI 配置」填入 yunzhiapi.cn Token 即可启用 AI 问答。

## 7. 备份与恢复

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

## 8. 升级

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

## 9. 健康检查

```bash
curl -s http://127.0.0.1:8080/api/health | jq
curl -s http://127.0.0.1:18080/api/judge/status | jq
```

## 10. 常见问题

**Q: 服务启动失败？**
A: 检查 `journalctl -u swoj -n 100`；常见原因：端口占用、`SWOJ_STATIC` 路径不存在、SQLite 文件权限错误。

**Q: AI 问答返回 503？**
A: 管理员未配置 Token，或未启用 AI 服务。进入管理后台 → AI 配置。

**Q: 提交返回 503？**
A: 未配置 `SWOJ_JUDGE_BIN` 或 go-judge 不可执行。

**Q: 内存占用过高？**
A: 调低 `SWOJ_WORKERS`（默认 4），或调低 `SWOJ_MEM_LIMIT_MB`。

**Q: 如何查看日志？**
A: `journalctl -u swoj -f`，或访问 `/admin/logs`。
