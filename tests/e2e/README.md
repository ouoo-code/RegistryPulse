# E2E 验收

需要一个已经启动的站点，默认地址为 `http://127.0.0.1`。首次使用：

```sh
cd tests/e2e
npm install --no-audit --no-fund
npx playwright install --with-deps chromium
npm test
```

可选环境变量：

- `E2E_BASE_URL`：被测站点地址。
- `ADMIN_API_TOKEN`：启用管理页测试；未设置时该测试会明确跳过。
- `E2E_START_SERVER=1`：由 Playwright 尝试执行 `docker compose up -d`，适用于 CI 前已有 `.env` 的场景。

E2E 不依赖第三方镜像站的实时状态；它只验证本项目页面、API 和本地 Compose 服务。
