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
~~~

单机模式下，API、Worker、前端、Nginx、PostgreSQL 和 Redis 运行在同一个 Compose 项目中。探测节点 Agent 的注册、心跳、任务拉取和结果上报接口已预留，可用于扩展多地区探测部署。


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
~~~

启动服务：

~~~bash
docker compose config
docker compose pull
docker compose up -d
docker compose ps
~~~

本版本按首次安装处理：API 第一次启动时会创建完整数据库结构并写入默认分类、测试镜像、系统设置和内置镜像源。项目不包含旧版本数据库迁移、回填、升级或回滚步骤；容器重启时会自动跳过已经完成的初始化。正式使用前请保留 PostgreSQL 持久化卷，并按需执行 `make backup`。

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
| postgres | 保存镜像源、任务、历史、故障和通知数据 |
| redis | 缓存、任务锁和调度协调 |

持久化卷：

- registrypulse_postgres-data
- registrypulse_redis-data

不要随意使用以下命令，否则会删除数据库卷：

~~~bash
docker compose down -v
~~~

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

## 许可证

本项目使用 [AGPL-3.0](https://www.gnu.org/licenses/agpl-3.0.html) 开源协议。

作者：

~~~text
ouoo-code  镜像脉动 · Registry Pulse
~~~

## 项目地址

GitHub：<https://github.com/ouoo-code/RegistryPulse>

欢迎提交 Issue、改进建议和 Pull Request。
