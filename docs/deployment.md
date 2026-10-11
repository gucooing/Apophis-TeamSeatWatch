# Docker 部署与 tag 发布

[返回项目首页](../README.md) · 简体中文 / [English](deployment.en.md)

容器镜像支持 **Linux x86_64 / linux/amd64**。镜像部署只需要 Docker Engine（含 Compose）；初始化容器会自动准备数据库密码、加密密钥和内部服务证书。管理与兑换入口使用 HTTP，可接到你已有的反向代理。

## 1. 通过 tag 发布

先把本次代码改动提交到自己的仓库，再推送 tag：

```bash
git push origin HEAD
git tag v1.0.0
git push origin v1.0.0
```

GitHub Actions 在 tag 推送时构建并发布：

- 镜像：`ghcr.io/gucooing/apophis-teamseatwatch:v1.0.0`。
- GitHub Release：与 tag 同名，附 `compose.yaml`、`env.example` 和部署命令。
- 镜像版本原样使用 Git tag，包括 `v` 前缀；不会自动移除前缀或改写版本号。
- fork 后工作流使用当前仓库的所有者和仓库名，镜像路径自动转为小写。

tag 需要同时符合 Docker tag 格式：1–128 个 ASCII 字母、数字、下划线、点或连字符，首字符不能是点或连字符。不支持带斜杠的版本号。工作流使用 GitHub 自带的 `GITHUB_TOKEN` 发布，不需要 Docker Hub 密钥。

如果需要匿名拉取镜像，首次发布后在 GitHub Packages 的包设置中将镜像可见性改为 **Public**。GitHub 首次创建容器包时默认是 Private；也可以保留私有并用具有 `read:packages` 权限的 classic PAT 先执行 `docker login ghcr.io`。参见 [GitHub Container Registry 文档](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)。

## 2. 直接通过 Docker 部署

下面以已经发布的 `v1.0.0` 为例；使用其他版本时替换下载地址里的 tag。无需克隆源码：

```bash
mkdir -p teamseatwatch
cd teamseatwatch
curl -fL https://github.com/gucooing/Apophis-TeamSeatWatch/releases/download/v1.0.0/compose.yaml -o compose.yaml
curl -fL https://github.com/gucooing/Apophis-TeamSeatWatch/releases/download/v1.0.0/env.example -o .env
```

在 `.env` 中确认镜像版本，并将管理员来源设置为浏览器实际使用的地址：

```dotenv
TSW_IMAGE=ghcr.io/gucooing/apophis-teamseatwatch:v1.0.0
COMPOSE_PROJECT_NAME=teamseatwatch
TSW_OWNER_ORIGINS=https://admin.example.com
TSW_BIND_ADDRESS=127.0.0.1
TSW_OWNER_PORT=18443
TSW_REDEEM_PORT=18444
```

`TSW_OWNER_ORIGINS` 不包含 `/owner/` 或末尾的斜杠。直接使用本机 HTTP 入口时，使用示例文件中的 `http://localhost:18443,http://127.0.0.1:18443`。修改管理端口时，也要相应修改来源里的端口。

```bash
docker compose up -d
docker compose logs control
```

首次启动自动创建管理员，日志中会出现：

```json
{"msg":"owner_initialized","username":"owner","password":"自动生成的随机密码"}
```

用日志里的密码登录。只有数据库中还没有管理员时才生成并输出密码；重启和更新会复用已有账号。自定义密码取消长度限制，需为非空值。自动生成的密码仍为 32 个随机字符。

| 入口 | 反向代理上游 / 本机地址 |
| --- | --- |
| 管理工作台 | `http://127.0.0.1:18443/owner/` |
| 用户兑换 | `http://127.0.0.1:18444/redeem/` |

数据库、管理员、业务数据及密钥分别保存在 Compose 持久卷中。对外入口不需要应用生成的 HTTPS 证书，也无需在浏览器导入 CA；control 与 gateway 的内部 mTLS 由初始化容器自动配置。

已有源码目录也可以部署同样的镜像：

```bash
cp deploy/.env.example .env
# 编辑 .env 中的镜像 tag 和管理员来源。
docker compose --env-file .env -f deploy/compose.yaml up -d
docker compose --env-file .env -f deploy/compose.yaml logs control
```

## 3. 接入已有反向代理

管理域名转发到 `http://127.0.0.1:18443`，兑换域名转发到 `http://127.0.0.1:18444`，保留 URL 路径。以已有 Nginx 站点中的 location 配置为例：

```nginx
# 管理域名的现有 server 内
location / {
    proxy_pass http://127.0.0.1:18443;
    proxy_set_header Host $http_host;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_read_timeout 300s;
    client_max_body_size 20m;
}

# 兑换域名的现有 server 内
location / {
    proxy_pass http://127.0.0.1:18444;
    proxy_set_header Host $http_host;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_read_timeout 300s;
}
```

外部域名和 HTTPS 证书由你的代理管理。来源配置为 HTTPS 时，登录 Cookie 带 `Secure`；仅配置 HTTP 来源时可直接通过 HTTP 登录。Origin 和 CSRF 校验仍按配置的浏览器地址执行。

如果反向代理位于另一台主机或独立容器，按其网络连接方式设置 `TSW_BIND_ADDRESS` 与上游主机地址。例如监听全部宿主机接口时使用 `TSW_BIND_ADDRESS=0.0.0.0`；默认只在宿主机回环地址发布两个端口。

## 4. ChatGPT 通信的客户端配置

网页请求和 Codex 请求分别配置。默认网页 UA 来自容器内实际安装的 Chromium 版本，浏览器与 HTTP 请求使用相同的 UA、语言及对应时区。项目专属 `Apophis-TeamSeatWatch/1`、固定 Windows Chrome 样本、旧网页构建号及固定观察标记已移除。

| 环境变量 | 默认与用途 |
| --- | --- |
| `TSW_CLIENT_USER_AGENT` | 留空时使用实际 Chromium UA；填写时同步用于网页 HTTP 请求和 Chromium |
| `TSW_CLIENT_ACCEPT_LANGUAGE` | 默认 `en-US,en;q=0.9`，同时设置浏览器语言与 HTTP Accept-Language |
| `TSW_CLIENT_TIMEZONE` | 默认 `UTC`；IANA 时区，如 `Asia/Taipei`，也用于空间发现的时区偏移 |
| `TSW_CLIENT_WEB_VERSION` | 可选真实网页版本；留空则不发送 Oai-Client-Version |
| `TSW_CLIENT_WEB_BUILD` | 可选真实网页构建号；留空则不发送 Oai-Client-Build-Number |
| `TSW_CLIENT_CODEX_USER_AGENT` | 默认 `codex_cli_rs/0.146.0`；必须保持 Codex 格式，独立于网页 UA |

在 `.env` 中按部署环境设置这些值，然后重新执行 `docker compose up -d`。自定义网页 UA 应与实际浏览器环境匹配。配置在同一进程内保持稳定，账号的 device ID 和 session ID 继续沿用保存的会话资料。

Codex OAuth、Token 交换、刷新与用量通信继续使用既有 Codex 客户端协议和 HTTP 传输，使用独立的 Codex UA；不会因为网页 UA 设置而切换为浏览器请求。OAuth 的 client ID、scope、originator 与 PKCE 校验保持协议要求。

这些改动减少固定请求头及版本样本的重复特征；Go HTTP 的 TLS 行为、出口 IP 和访问模式仍可被上游观察，不能保证不被识别或触发风控。

## 5. 日常维护

在下载的 Compose 文件所在目录执行：

```bash
docker compose ps
docker compose logs --tail 100 control gateway
docker compose stop
docker compose up -d
```

忘记管理员密码时，可使用任意非空的新密码重置。旧会话随之失效：

```bash
docker compose run --rm --no-deps -e TSW_OWNER_LOGIN=owner -e TSW_OWNER_PASSWORD='new-password' cli owner-reset
```

更新时，在同一目录修改 `.env` 的 `TSW_IMAGE` 为新的 tag；保留 `COMPOSE_PROJECT_NAME` 和持久卷：

```bash
docker compose stop control gateway
docker compose pull
docker compose up -d
```

迁移前按已有备份流程保存数据库及全部配套 secrets 卷，恢复时使用同一份备份中的数据库和密钥。保持部署目录与项目名一致以复用原持久卷。

## 6. 从源码构建

源码构建使用同一份 Compose 配置，不需要 Python 安装工具：

```bash
git clone https://github.com/gucooing/Apophis-TeamSeatWatch.git
cd Apophis-TeamSeatWatch
docker build --platform linux/amd64 -t teamseatwatch:local .
cp deploy/.env.example .env
# 将 .env 中的 TSW_IMAGE 改为 teamseatwatch:local，并设置管理员来源。
docker compose --env-file .env -f deploy/compose.yaml up -d
docker compose --env-file .env -f deploy/compose.yaml logs control
```

初始化、HTTP 入口、自动管理员密码和客户端配置与镜像部署相同。原 Python 安装工具及旧 HTTPS 代理方案已移除。
