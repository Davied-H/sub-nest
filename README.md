# Sub Nest

Sub Nest 是一个自托管订阅聚合器：后台维护多个上游订阅源或本地订阅文件，对外输出稳定地址，例如 `/s/main`。

## 功能

- 管理多个订阅源，支持 URL 拉取和直接上传订阅文件。
- 自动识别 Clash/Mihomo YAML、base64 订阅和明文分享链接列表。
- 解析 SS、VMess、VLESS、Trojan、Hysteria/Hysteria2 分享链接。
- 合并节点、去重、过滤、正则重命名和地区自动分组。
- 可通过 Mihomo/Clash 核心真实测试节点出口 IP，并按出口地区分组。
- 节点测试失败时标记不可用，公开订阅默认排除不可用节点。
- 输出 Clash/Mihomo YAML 或 base64 分享链接列表，并支持多格式下载。
- 刷新时显示状态和进度条；上游失败时保留上一次成功缓存。
- 支持 admin 超级管理员和多用户独立空间；用户通过一次性授权码注册自己的 token。
- 后台 token 访问保护，列表中隐藏订阅链接敏感部分。
- 支持在后台修改管理员 token，并可设置用户 token 保护公开订阅地址。
- 每个订阅源可独立配置剩余流量查询，支持订阅响应头、正文正则和自定义 HTTP。
- 支持 JSON 备份与恢复。

## 本地运行

```bash
cd sub-nest/web
npm install
npm run build

cd ..
go run ./cmd/subagg -addr :8080
```

打开 `http://localhost:8080`，首次进入会要求设置管理 token。初始化后的 admin 可以在“用户”页创建一次性授权码，其他用户用授权码注册自己的 token 后独立使用。

登录后可在“设置”里修改两类 token：

- 管理员 token：用于登录后台和管理配置。
- 用户 token：可选，用于保护 `/s/<slug>` 公开订阅地址；未设置时公开订阅保持无需 token 访问。

## 环境变量

- `SUBAGG_ADDR`：监听地址，默认 `:8080`
- `SUBAGG_DATA`：配置文件路径，默认 `data/config.json`
- `SUBAGG_STATIC`：前端静态文件目录，默认 `web/dist`
- `SUBAGG_GEOIP_DB`：可选 GeoIP 数据库路径；未设置时会尝试 `data/GeoLite2-Country.mmdb`
- `SUBAGG_MIHOMO_BIN`：可选 Mihomo/Clash 核心路径；未设置时会查找 `mihomo`、`clash-meta`、`clash`

## Docker 一键部署

推荐直接使用内置 Mihomo 的 GitHub Container Registry 镜像：

```bash
mkdir -p sub-nest
cd sub-nest
curl -fsSLO https://github.com/Davied-H/sub-nest/releases/latest/download/compose.yaml
docker compose up -d
```

或者手动创建 `compose.yaml`：

```yaml
services:
  sub-nest:
    image: ghcr.io/davied-h/sub-nest:latest
    ports:
      - "18788:8080"
    volumes:
      - ./data:/data
```

访问后台：

```text
http://<HOST>:18788
```

首次进入会要求设置管理 token。所有配置会持久化在 `./data/config.json`。admin 可以创建一次性授权码，用户注册后会拥有独立订阅源、公开订阅和备份空间。

## Docker 镜像维护

从源码构建本地镜像：

```bash
docker build -t sub-nest:local .
docker compose -f compose.local.yaml up -d --build
```

发布镜像：

```text
ghcr.io/davied-h/sub-nest:latest
ghcr.io/davied-h/sub-nest:v0.1.0
ghcr.io/davied-h/sub-nest:slim
ghcr.io/davied-h/sub-nest:v0.1.0-slim
```

`latest` 和版本镜像内置 Mihomo，可直接进行真实出口检测。`slim` 和版本 `slim` 镜像不内置 Mihomo，适合想自己挂载 core 的用户：

```yaml
services:
  sub-nest:
    image: ghcr.io/davied-h/sub-nest:slim
    ports:
      - "18788:8080"
    environment:
      SUBAGG_MIHOMO_BIN: /usr/local/bin/mihomo
    volumes:
      - ./data:/data
      - ./bin/mihomo:/usr/local/bin/mihomo:ro
```

没有 core 时应用仍可运行，刷新订阅源时会跳过真实出口检测。

注意：`./bin/mihomo` 必须匹配容器架构。例如 x86_64 NAS 需要 Linux amd64，不要把 macOS `darwin arm64` 的本地开发版同步到 NAS，否则节点检测会报 `exec format error`。

如果要本地手动构建同样的镜像矩阵：

```bash
docker build -t ghcr.io/davied-h/sub-nest:latest .
docker build --target slim -t ghcr.io/davied-h/sub-nest:slim .
```

## NAS 部署

NAS 项目目录：

```text
/mnt/user/appdata/sub-nest
```

部署命令：

```bash
cd /mnt/user/appdata/sub-nest
docker compose build
docker compose up -d
docker compose ps
```

从本地同步代码到 NAS 时，不要覆盖持久化数据和 NAS 专用二进制：

```bash
rsync -av --delete \
  --exclude ".git" \
  --exclude "web/node_modules" \
  --exclude "data/" \
  --exclude "bin/" \
  ./ unraid-nas:/mnt/user/appdata/sub-nest/
```

访问地址：

```text
http://<NAS_IP>:18788
```

## 公开订阅

创建 slug 为 `main` 的公开订阅后，客户端订阅：

```text
http://<HOST>:18788/s/main
```

普通用户的公开订阅地址带用户命名空间：

```text
http://<HOST>:18788/u/<USER_SLUG>/s/main
```

多格式下载：

```text
/s/main?format=mihomo&download=1
/s/main?format=clash&download=1
/s/main?format=base64&download=1
```

## 流量查询

每个订阅源都可以在编辑面板中配置流量查询方式：

- 读取订阅响应头：解析常见的 `subscription-userinfo` 响应头。
- 订阅正文正则：从订阅响应正文里提取剩余、总量、上传、下载和到期时间。
- 自定义 HTTP：请求独立接口，并用 JSON 路径或正则提取流量字段。

流量查询失败不会影响订阅源节点刷新；失败信息会保存在订阅源列表和编辑面板中。
