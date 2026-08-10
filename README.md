# Registry Pulse

**镜像脉动 · Registry Pulse** 是一个面向 Docker Hub、GHCR、Quay、MCR、Kubernetes Registry 及其他 OCI/Registry 镜像代理站的实时监控平台。

项目通过真实的 Registry HTTP 探测，记录镜像源的可用性、响应时间、失败阶段和历史结果，并提供配置生成器与管理后台，帮助开发者和运维人员选择、验证和维护容器镜像加速源。

> 第三方镜像源的状态只代表本次实际探测结果，不代表其长期可用性或服务承诺。


## 特点

- 真实探测 Registry，不使用静态演示或随机模拟数据
- 支持 DNS、TCP、TLS、Registry API、Bearer Token、Manifest 和受限 Blob 下载探测
- 支持 Docker Hub、GHCR、Quay、MCR、Kubernetes、GCR、Elastic、NVCR 和自定义 Registry
- 保存检测结果、故障事件和阶段错误，支持详情页与历史查询
- 可配置正常检测间隔、异常重试间隔、超时、并发和状态判定阈值
- 首页支持分类页签、搜索、状态筛选、排序、表头调整和镜像源详情
- 支持 Docker daemon.json、Podman、Containerd 等配置生成
- 支持中文/英文界面、浅色/深色主题和响应式布局
- 管理后台支持镜像源、分类、任务、测试镜像、通知、通知规则和系统设置
- 支持 Gotify、Webhook、SMTP 通知
- PostgreSQL 保存业务数据，Redis 用于缓存、调度和任务锁
- Docker Compose 一键部署，数据使用独立持久化卷
- 提供健康检查、Prometheus 指标、备份和恢复脚本

## 页面

| 页面 | 地址 | 说明 |
| --- | --- | --- |
| 首页 | / | 镜像源总览、分类、状态统计和筛选 |
| 分类页 | /status/:category | 查看指定 Registry 类别 |
| 镜像源详情 | /source/:id | 当前状态、历史检测、响应趋势和故障记录 |
| 配置生成器 | /configure | 生成 Docker、Podman、Containerd 等配置 |
| 教程 | /tutorial | Docker、Podman、Containerd 和常见问题 |
| 关于 | /about | 软件信息、协议和项目说明 |
| 管理后台 | /admin | 登录后管理镜像源和系统功能 |

## 架构

~~~mermaid
flowchart LR
    Browser[浏览器] --> Nginx[Nginx]
    Nginx --> Frontend[Vue 前端]
    Nginx --> API[Go API]
    API --> PostgreSQL[(PostgreSQL)]
    API --> Redis[(Redis)]
    Worker[Go Worker] --> PostgreSQL
    Worker --> Redis
    Worker --> Registry[外部容器镜像 Registry]
    Agent[可选 Probe Agent] --> API
    Agent --> Registry
    Proxy[独立 registry-proxy] --> Redis
    Proxy --> Registry
~~~

单机模式下，API、Worker、前端、Nginx、PostgreSQL、Redis 和独立的 registry-proxy 运行在同一个 Compose 项目中。探测节点 Agent 的注册、心跳、任务拉取和结果上报接口已预留，可用于扩展多地区探测部署。


![前台界面](rp-1.png)

![后台管理](rp-2.png)



## 快速开始

### 环境要求

- Docker Desktop 或 Docker Engine
- Docker Compose v2
- Git
- Windows、Linux 或 macOS

### Docker Compose 部署

Linux/macOS：

~~~bash
cp .env.example .env
~~~

Windows PowerShell：

~~~powershell
Copy-Item .env.example .env
~~~

编辑 .env，至少修改管理员密码和安全密钥：

~~~env
ADMIN_USERNAME=admin
ADMIN_PASSWORD=请设置高强度密码
SESSION_SECRET=请设置随机密钥
JWT_SECRET=请设置随机密钥
ENCRYPTION_KEY=请设置随机密钥
CREDENTIAL_ENCRYPTION_KEY=请设置32字节随机密钥（用于加密镜像仓库凭证）
~~~

启动服务：

~~~bash
docker compose config
docker compose pull
docker compose up -d
docker compose ps
~~~

本版本按首次安装处理：API 第一次启动时会创建完整数据库结构并写入默认分类、测试镜像、系统设置和内置镜像源。项目不包含旧版本数据库迁移、回填、升级或回滚步骤；容器重启时会自动跳过已经完成的初始化。正式使用前请保留 PostgreSQL 持久化卷，并按需执行 `make backup`。如果重新部署时需要清空环境，应先停止服务并明确删除本项目的 PostgreSQL/Redis 数据卷；这会永久删除数据。

应用发布版本存储在项目根目录的 `VERSION` 文件中。Compose 变量
`REGISTRYPULSE_VERSION=latest` 用于选择 Docker Hub 镜像通道；它不是应用界面显示的版本号。

访问：

- 前台：<http://localhost>
- 管理后台：<http://localhost/admin>

健康检查：

~~~bash
curl -f http://localhost/health
curl -f http://localhost/api/v1/health
curl -f http://localhost/api/v1/public/summary
~~~

停止和日志：

~~~bash
docker compose down
docker compose logs -f api worker
~~~

## Docker 服务和持久化

Compose 项目技术名为 **registrypulse**。

| 服务 | 作用 |
| --- | --- |
| nginx | 对外提供 80 端口并代理前端/API |
| frontend | 构建并提供 Vue 静态前端 |
| api | Go REST API、认证、管理和公开查询 |
| worker | 调度探测任务并保存检测结果 |
| proxy | 在 10800 端口提供 Docker Registry 只读实时转发 |
| postgres | 保存镜像源、任务、历史、故障和通知数据 |
| redis | 缓存、任务锁和调度协调 |

持久化卷：

- registrypulse_postgres-data
- registrypulse_redis-data

不要随意使用以下命令，否则会删除数据库卷：

~~~bash
docker compose down -v
~~~

### Registry Proxy 实时转发

`registry-proxy` 是独立的数据面入口，默认监听宿主机 `10800` 端口。第一阶段固定为 Docker Hub 类别，按 API/Worker 写入 Redis 的健康路由快照选择启用、非维护、最近探测成功的源，并在上游连接或 5xx 失败时有限度切换备用源。

代理当前只允许 `GET`、`HEAD` 和 Docker Registry 的 `/v2/`、Manifest、Blob 路径，默认关闭 push、delete 和 upload。开发环境可以使用：

~~~text
http://localhost:10800
~~~

生产环境应使用真实域名和受信任的 HTTPS 证书；`https://localhost:10800` 不是自动提供的 TLS 入口。

代理不会缓存镜像内容：Manifest、Manifest List、config blob、layer blob 和 OCI Artifact 都使用流式实时转发，不写入 PostgreSQL、Redis、宿主机目录或容器文件系统。Redis 只保存路由/健康快照，允许 HTTP 连接池和短期进程状态。大响应不会为了缓存命中而改变 digest、Range、MediaType、Content-Length 或状态码。

可调整的环境变量包括 `PROXY_CATEGORY_ID`、`PROXY_UPSTREAMS`、`PROXY_ROUTE_MAX_AGE`、`PROXY_MAX_CONCURRENT`、`PROXY_MAX_RANGE_BYTES`、`PROXY_MAX_MANIFEST_BYTES` 和 `PROXY_REDIRECT_HOSTS`。未配置健康快照时，显式配置的 `PROXY_UPSTREAMS` 作为回退上游；不能通过请求参数改变上游地址。

首次实现暂不把认证密码放入路由快照，也不把客户端 `Authorization` 原样转发到备用源。需要私有仓库认证时，应先在凭证配置中完成 host/类别绑定，再扩展代理的 host-bound Bearer/Basic 凭证模块。

## 镜像源探测

默认流程包括 DNS、TCP、TLS、Registry /v2/ API、Bearer Token、Manifest 和受限 Blob Range 下载。

记录内容包括总响应时间、各阶段耗时、HTTP 状态码、远端地址、Manifest 信息、Blob 首字节时间、下载速度、错误阶段、最后检测时间和故障事件。

状态包括：

- **运行**：核心 Registry 能力可用
- **缓慢**：功能可用但响应或下载速度达到慢速阈值
- **离线**：连续检测失败或无法访问
- **维护**：管理员手动设置，自动探测不会覆盖
- **未知**：尚未获得足够检测结果

默认调度设置：

~~~env
PROBE_INTERVAL=30m
PROBE_RETRY_INTERVAL=3m
PROBE_MAX_CONCURRENCY=20
PROBE_TIMEOUT=10s
PROBE_DOWNLOAD_BYTES=2097152
~~~

正常轮询和异常重试相互独立。每个正常检测周期都会保存一条记录；同一故障期间的异常重试会根据状态是否变化进行去重，避免日志快速膨胀。

### Docker Pull 探测说明

如果在后台选择 `docker pull` 探测，但提示：

```text
探测配置测试失败: docker pull disabled
```

表示项目没有启用真实 Docker Pull。默认配置为：

```env
ENABLE_REAL_DOCKER_PULL=false
```

如确实需要启用，请修改 `.env`：

```env
ENABLE_REAL_DOCKER_PULL=true
```

同时 API/Worker 容器必须挂载宿主机 Docker Engine Socket：

```yaml
volumes:
  - /var/run/docker.sock:/var/run/docker.sock
```

Windows Docker Desktop 通常使用：

```yaml
- //var/run/docker.sock:/var/run/docker.sock
```

Docker Socket 权限很高，启用后应用可以调用宿主机 Docker，存在安全风险。普通镜像站连通性检测建议使用“注册表探测”或“Manifest 探测”，它们更安全、速度也更快。

## 配置生成器

支持 Docker daemon.json、1Panel Docker 配置、Podman registries.conf、Containerd 配置，以及镜像前缀拉取和重新标记命令。

Docker Hub 示例：

~~~json
{
  "registry-mirrors": [
    "https://mirror.example.com"
  ]
}
~~~

非 Docker Hub 示例：

~~~bash
docker pull ghcr.example.com/user/image:tag
docker tag ghcr.example.com/user/image:tag ghcr.io/user/image:tag
~~~

## 管理后台和通知

管理后台位于 /admin，支持镜像源、分类、测试镜像、手动探测、任务、历史、故障事件、系统设置、密码、可选 TOTP、导入导出、通知和通知规则管理。

通知通道支持 Gotify、Webhook 和 SMTP Email。模板变量包括：

~~~text
{source_name}
{event}
{message}
{status}
~~~

生产环境不要把管理员密码、Token、SMTP 密码或 Webhook 密钥提交到 Git。

### 凭证配置

凭证配置用于为需要认证的镜像源探测提供登录信息，支持三种认证类型：

| 认证类型 | 请求方式 | 适用场景 | 用户名 | 密钥 |
| --- | --- | --- | --- | --- |
| Basic authentication | `Authorization: Basic ...` | 私有 Registry、企业内部仓库 | 填写 Registry 用户名 | 填写密码或访问密码 |
| Bearer Token | `Authorization: Bearer ...` | GHCR PAT、私有 Registry Token、云厂商 Token | 通常留空 | 填写 Bearer Token 或 PAT |
| Token | 当前实现同样使用 Bearer | 泛指访问 Token 或 PAT | 通常留空 | 填写访问 Token |

`Bearer Token` 和 `Token` 在当前版本中的实际 HTTP 发送方式相同，区别主要是配置名称。已有明确 Bearer 规范的仓库建议选择 `Bearer Token`。

凭证匹配范围可以选择：

1. **镜像源**：只匹配一个具体镜像源，优先级最高。
2. **注册表域名**：匹配同一域名下的镜像源，例如 `ghcr.io` 或 `registry.example.com`。只填写域名，不要填写 `https://`、路径或 `/v2/`。
3. **镜像源类别**：匹配整个 GHCR、MCR 等类别。

匹配优先级为：

```text
特定镜像源 > 注册表域名 > 镜像源类别
```

示例：

```text
私有 Registry：Basic authentication，用户名 admin，密钥为登录密码，匹配 registry.example.com
GHCR：Bearer Token，用户名留空，密钥为 GitHub Personal Access Token，匹配 ghcr.io 或 GHCR 类别
```

测试镜像中的认证策略与凭证配置不同：

- **匿名访问**：不要求凭证。
- **可选认证**：有匹配凭证时使用，没有凭证时继续匿名探测。
- **必须认证**：没有匹配凭证时直接判定认证失败。

凭证密钥不会明文保存，使用 `CREDENTIAL_ENCRYPTION_KEY` 加密存储。生产环境必须设置随机的 32 字节加密密钥，并妥善备份。

## API 与可观测性

API 前缀为 /api/v1。

~~~text
GET  /api/v1/health
GET  /api/v1/public/summary
GET  /api/v1/public/categories
GET  /api/v1/public/sources
GET  /api/v1/public/sources/{id}
GET  /api/v1/public/sources/{id}/history
GET  /api/v1/public/sources/{id}/incidents
POST /api/v1/public/config-generator/render
POST /api/v1/auth/login
GET  /api/v1/admin/sources
POST /api/v1/admin/sources/{id}/probe
GET  /api/v1/admin/tasks
GET  /api/v1/admin/results
GET  /api/v1/admin/incidents
GET  /api/v1/admin/settings
~~~

健康检查和指标：

~~~text
GET /health/live
GET /health/ready
GET /metrics
~~~

## 安全设计

项目包含 Registry URL 校验、SSRF 防护、DNS 重定向校验、Argon2id 密码哈希、Session 认证、登录限流、可选 TOTP、请求 ID、审计日志、敏感信息脱敏、请求超时和并发限制。

默认安全配置：

~~~env
ALLOW_PRIVATE_TARGETS=false
ALLOW_INSECURE_HTTP=false
PUBLIC_API_ENABLED=true
~~~

## 备份、恢复和测试

~~~bash
make backup
make restore DIR=backups/YYYY-MM-DD_HH-mm-ss
make test
make test-backend
make test-frontend
make test-e2e
make lint
make validate
make compose-smoke
~~~

探测器测试使用本地模拟服务和固定响应，不依赖第三方镜像站实时可用性。

## 目录结构

~~~text
RegistryPulse/
├── backend/                 Go API、Worker、Probe Agent、首次安装数据库结构
├── frontend/src/            Vue 页面、组件、API 和国际化
├── deploy/                  Nginx 与部署脚本
├── tests/                   API、Compose、前端和 E2E 测试
├── .env.example             环境变量模板
├── docker-compose.yml       Docker Compose 定义
├── Makefile                 常用开发命令
├── LICENSE                  AGPL-3.0 许可证
└── README.md                项目说明
~~~

## 当前边界与后续规划

当前版本重点完成了单机部署、真实镜像源探测、状态历史、配置生成、管理后台和通知能力。

后续可继续完善多地区探测节点的生产级部署、更多 Registry 厂商的专用鉴权策略、历史数据聚合、更完整的容器运行时 Pull 探测隔离、细粒度 RBAC、Grafana 模板和更完整的 OpenAPI 客户端。

### Registry Proxy 后台管理

管理员可以在“设置 → Registry Proxy”中管理代理控制面。页面显示代理进程心跳、就绪状态、实际监听端口、配置端口、镜像源类别和当前健康候选源数量。启用开关支持热更新；关闭后进程仍保持运行，但 Registry 请求会返回 `PROXY_DISABLED`。

镜像源类别、路由快照有效期、失败冷却、最大并发、Range 上限和 Manifest 上限会保存到 PostgreSQL，并通过 Redis 控制快照下发，修改后无需替换容器即可生效。监听端口由 Docker Compose 管理，不属于后台运行时设置；如需修改对外端口，请直接编辑 `docker-compose.yml` 中 `proxy.ports` 和 `PROXY_HTTP_PORT`，然后重启 `registry-proxy` 服务。

“流量处理方式”支持两种模式：

- **转发流量**：代理读取并流式转发上游响应，适合需要统一鉴权、故障切换和请求控制的场景。
- **重定向流量**：代理只选择健康源并返回 `307 Location`，Docker 客户端随后直接从上游读取镜像内容，主体流量不会经过本机；适合公开可直接访问的镜像源。该模式不提供代理侧的统一鉴权和响应级故障切换。

两种模式都只允许受控的 Registry/OCI 路径，并继续执行目标地址校验；重定向不是开放任意 URL 的跳转服务。

## 许可证

本项目使用 [AGPL-3.0](https://www.gnu.org/licenses/agpl-3.0.html) 开源协议。

作者：

~~~text
ouoo-code  镜像脉动 · Registry Pulse
~~~

## 项目地址

GitHub：<https://github.com/ouoo-code/RegistryPulse>

欢迎提交 Issue、改进建议和 Pull Request。
