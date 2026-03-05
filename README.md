# 🐾 NekoIPinfo

[![Go Version](https://img.shields.io/badge/Go-1.16+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)
[![Author Blog](https://img.shields.io/badge/Blog-nekopara.uk-ff69b4.svg)](https://www.nekopara.uk)

**NekoIPinfo** 是一个专为极致性能而生的开源 IPv4 归属地查询服务。带有一点点可爱的猫娘风格喵 🐱~

它采用 **Nginx 静态托管 + Go 纯净 API + SQLite B-Tree 索引** 的现代化架构，专为低配置（如 2vCPU 1GB RAM）轻量云服务器深度优化。

经测试，在老旧的双核 CPU （测试型号为 i5-2410M ，性能与主流轻量服务器类似）上也能扛住 **5000+ QPS**，内存占用保持在 **40MB** 以下（默认不开启数据库内存缓存的情况下），是小微型服务器构建 IP 查询服务的究极解法喵！🚀

---

## 📡 部署示例

你可以访问我搭建的服务来看看这个项目的效果如何喵～

**我的查询实例：** [NekoIPinfo Demo](https://ip.nekopara.uk)


---

## 📐 架构设计

本项目的核心在于高效的 **B-Tree 索引二分查找**。请求流向如下：
`访客 -> Nginx (限流/HTTPS卸载) -> Go API (校验/内存逻辑) -> SQLite (索引定位)`

---

## ✨ 核心特性

* ⚡ **极致性能**：利用 SQLite B-Tree 索引，将百万级数据的查询开销从 $O(N)$ 降至 $O(\log N)$，查询耗时仅为微秒级。
* 🪶 **超低损耗**：原生 Go 编写，无第三方 Web 框架负担，部署后几乎不占用系统资源。
* 🛡️ **安全加固**：强类型 IPv4 解析校验，配合参数化 SQL 查询，从根源杜绝 SQL 注入与非法请求。
* 🌐 **前后端分离**：前端采用纯净 HTML/CSS（本地化，不依赖外部 CDN），后端提供纯粹 JSON 响应，适配各种反代场景。
* 🚦 **内建防御**：配套提供的 Nginx 配置模板自带频率限制，有效防止 API 被恶意爆破。

---

## 💻 API 调用说明

后端提供纯净的 RESTful API 接口，返回标准 JSON 格式数据。

### 1. 查询指定 IP
**GET** `/ipinfo?ip={IPv4地址}`

**请求示例：** `/ipinfo?ip=8.8.8.8`
**返回响应：**
```json
{
    "code": 200,
    "msg": "success",
    "data": {
        "ip": "8.8.8.8",
        "country": "美国",
        "province": "加利福尼亚州",
        "city": "圣克拉拉",
        "isp": "Google",
        "latitude": "37.386052",
        "longitude": "-122.083851"
    }
}

```

### 2. 获取访客自身 IP (自动检测)

如果在请求时不传递 `ip` 参数，API 将自动解析并返回**请求者本身**的 IP 信息！
**GET** `/ipinfo`

**Response:**

```json
{
    "code":200,
    "msg":"success",
    "data":{
        "ip":"119.8.185.128",
        "country":"新加坡",
        "province":"新加坡",
        "city":"",
        "isp":"huawei.com",
        "latitude":"1.352083",
        "longitude":"103.819836"
    }
}

```

---

## 🚀 部署指南

### 1. 数据库准备与性能优化

为了让查询速度飞起来，你需要创建一个经过索引优化的 `ip_info.db` 文件。

**数据表结构：**
```sql
CREATE TABLE ip_info (
    network_start INTEGER NOT NULL, -- IP 段起始值
    network_end INTEGER NOT NULL,   -- IP 段结束值
    ip_info_json TEXT NOT NULL      -- 归属地 JSON 详情
);

```

**插入数据：**
```sql
INSERT INTO ip_info (network_start, network_end, ip_info_json)
VALUES (2163295488, 2163295743, '{"country":"美国","province":"俄亥俄州","city":"辛辛那提","isp":"ntt.com","latitude":"39.103118","longitude":"-84.512019"}');
```

字段若无有效值，将以空字符串 `""` 填充，确保 JSON 结构稳定。

示例数据表样式：

| `network_start` (整数) | （→ 起始 IP） | `network_end` (整数) | （→ 结束 IP） | `ip_info_json` (TEXT) |
|------------------------|----------------------|----------------------|--------------------|------------------------|
| `0`                    | `0.0.0.0`            | `16777215`           | `0.255.255.255`    | `{"country":"特殊地址","province":"本网络","city":"This Network","isp":"This Network","latitude":"0","longitude":"0"}` |
| `16777216`             | `1.0.0.0`            | `16777471`           | `1.0.0.255`        | `{"country":"CLOUDFLARE.COM","province":"CLOUDFLARE.COM","city":"","isp":"","latitude":"","longitude":""}` |
| `16842752`             | `1.0.1.0`            | `16843519`           | `1.0.3.255`        | `{"country":"中国","province":"福建","city":"","isp":"电信","latitude":"25.908899","longitude":"118.125809"}` |
| `16843776`             | `1.0.4.0`            | `16845823`           | `1.0.7.255`        | `{"country":"澳大利亚","province":"维多利亚州","city":"墨尔本","isp":"gtelecom.com.au","latitude":"-37.813627","longitude":"144.963057"}` |
| `16846848`             | `1.0.8.0`            | `16850943`           | `1.0.15.255`       | `{"country":"中国","province":"广东","city":"","isp":"电信","latitude":"22.858749","longitude":"113.419327"}` |



**🔥 关键步骤（开启性能外挂）：**
导入数据后，请务必建立索引：
```sql
CREATE INDEX idx_network_start ON ip_info (network_start);

```

**⚠️ 警告：** 没有这行命令，性能将下降 500 倍以上，后果很严重喵！

> **💡 数据来源提示：** 你可以使用 [DB-IP](https://db-ip.com/) 或 [MaxMind](https://dev.maxmind.com/geoip) 的免费离线库，将其转换为上述格式写入 SQLite。本项目 `db_example` 文件夹中提供了一个内网 IP 的示例库 `lan_sample.db` 和一个空库模板 `empty.db` 供你测试喵。

### 2. 后端编译与启动

```bash
# 克隆仓库
git clone [https://github.com/Chocola-X/NekoIPinfo.git](https://github.com/Chocola-X/NekoIPinfo.git)
cd NekoIPinfo

# 编译生成二进制文件
go build -o neko-ip-api main.go

# 启动服务
./neko-ip-api -port 8080 -db ./ip_info.db

```

当然，也可以直接下载编译好的二进制文件直接启动。

**参数说明：**

* `-port`: 监听端口（默认 8080）
* `-db`: 数据库文件路径（默认 ./ip_info.db）
* `-mem=true`: 开启内存缓存模式，整个数据库将加载到内存，内存占用 ≈ 数据库文件大小 × 2~3 倍，请根据实际数据量评估。默认关闭
* `-log=true`: 控制台打印输出API调用日志，方便进行API调用行为监控。默认关闭

### 3. Nginx 环境配置

将前端文件（`static`文件夹的内容）放置于 `/var/www/nekoipinfo`，并参考以下配置：

无限流版本：

```nginx
server {
    listen 443 ssl;
    server_name ip.nekopara.uk;
    
    # 仅当你在使用 CDN 时开启以下两行获取真实 IP
    # set_real_ip_from 0.0.0.0/0;
    # real_ip_header X-Forwarded-For;
    
    # SSL证书和密钥路径
    ssl_certificate /data/certs/nekopara_uk.pem;
    ssl_certificate_key /data/certs/nekopara_uk.key;

    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_prefer_server_ciphers on;

    location / {
        root /var/www/nekoipinfo;
        index index.html;
        gzip on;
        gzip_types text/plain text/css application/json application/javascript;
    }
    
    location /ipinfo {
        proxy_pass http://127.0.0.1:8080;
        # 必须透传真实 IP
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header Host $http_host;
    }
    
}
```
带限流版本：
```
# 定义针对 API 的高频限流区：200请求/秒
limit_req_zone $binary_remote_addr zone=api_limit:10m rate=200r/s;
# 定义针对 静态页面 限流区：50请求/秒
limit_req_zone $binary_remote_addr zone=static_limit:10m rate=50r/s;
server {
    listen 443 ssl;
    server_name ip.nekopara.uk;
    
    # 仅当你在使用 CDN 时开启以下两行获取真实 IP
    # set_real_ip_from 0.0.0.0/0;
    # real_ip_header X-Forwarded-For;
    
    # SSL证书和密钥路径
    ssl_certificate /data/certs/nekopara_uk.pem;
    ssl_certificate_key /data/certs/nekopara_uk.key;

    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_prefer_server_ciphers on;

    # --- 通用网页路由 (返回 HTML 错误页) ---
    location / {
        limit_req zone=static_limit burst=50 nodelay;

        root /var/www/nekoipinfo;
        index index.html;
        gzip on;
        gzip_types text/plain text/css application/json application/javascript;
        
        # 捕获默认的 503，转换为 429 并返回 HTML
        error_page 503 =429 /429.html;
    }
    
    # --- API 接口路由 (返回 JSON 错误页) ---
    location /ipinfo {
        # 使用新定义的 api_limit (200r/s)
        limit_req zone=api_limit burst=400 nodelay;

        proxy_pass http://127.0.0.1:8080;
        
        # 必须透传真实 IP
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header Host $http_host;

        # 捕获默认的 503，转换为 429 并跳转到 @api_limit_json
        # 注意这里写的是 503，因为 limit_req 原生抛出的是 503
        error_page 503 =429 @api_limit_json;
    }
    
    # --- 内部命名 Location：直接返回 JSON 字符串 ---
    # 当上面捕获到 503 后，Nginx 会内部跳转到这里，并以 429 状态码返回内容
    location @api_limit_json {
        internal;
        default_type application/json;
        
        # 直接返回 429 状态码和 JSON 内容
        return 429 '{"code": 429, "msg": "请求速度过快，请稍后重试", "data": null}\n';
    }

    # --- 内部 Location：处理 HTML 错误页 ---
    # 供 location / 使用
    location = /429.html {
        internal;
        root /etc/nginx/error_pages;
        add_header Retry-After 1 always;
        add_header Content-Type text/html always;
    }
}
```

**提示：** 如果你的服务器位于 Cloudflare 等 CDN 背后，需要在server块内设置 `set_real_ip_from 0.0.0.0/0;` 和 `real_ip_header X-Forwarded-For;`。如果你直接暴露在公网，不要进行配置，以免被伪造 IP！

---

## 🛠️ 后台常驻 (Systemd)

创建服务文件 `/etc/systemd/system/neko-ip.service`:

```ini
[Unit]
Description=NekoIPinfo API Service
After=network.target

[Service]
ExecStart=/path/to/neko-ip-api -port 8080 -db /path/to/ip_info.db
WorkingDirectory=/path/to/project
Restart=always
User=www-data

[Install]
WantedBy=multi-user.target

```

当然，也可以使用**TMUX**挂着服务端喵～

---

## 🔗 链接

* **GitHub**: [Chocola-X/NekoIPinfo](https://github.com/Chocola-X/NekoIPinfo)
* **Blog**: [nekopara.uk](https://www.nekopara.uk)

---

## 📄 开源协议 (License)

本项目采用 **GNU Affero General Public License v3.0 (AGPL-3.0)** 协议开源。

**这意味着：** 如果你对本项目进行了修改并在云端运行了该服务，你**必须**向访问者公开你修改后的完整源代码。开源精神万岁喵！🐾

