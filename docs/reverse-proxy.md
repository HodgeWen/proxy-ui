# Reverse Proxy Configuration

s-ui 面板仅提供 HTTP 服务，HTTPS 通过反向代理（Nginx 或 Caddy）实现。监听端口可通过配置文件的 `addr` 或环境变量 `ADDR` 配置。

## Nginx

```nginx
server {
    listen 443 ssl;
    server_name example.com;
    ssl_certificate /path/to/fullchain.pem;
    ssl_certificate_key /path/to/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

**重要：** 当面板置于 HTTPS 反向代理后时，请设置环境变量 `FORCE_HTTPS=true`，以使 session cookie 的 Secure 标志生效，确保浏览器通过 HTTPS 发送 cookie。

## Caddy

```caddyfile
example.com {
    reverse_proxy :8080
}
```

Caddy 会自动申请 HTTPS 证书并代理到面板。代理目标端口（默认 8080）需与面板的 `addr` 一致。

**重要：** 同样需设置 `FORCE_HTTPS=true`，以便 session cookie 通过 HTTPS 正确发送。
